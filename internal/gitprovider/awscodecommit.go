// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	cctypes "github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/fluxcd/pkg/auth"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// AWSCodeCommitProvider implements the gitprovider.Interface for AWS AWSCodeCommit.
type AWSCodeCommitProvider struct {
	Client   *codecommit.Client
	Remote   *gogit.Remote
	Auth     *githttp.BasicAuth
	Region   string
	RepoName string
	RepoURL  string
}

// NewAWSCodeCommitProvider creates a new AWSCodeCommit provider from the given options.
// The credsProvider is an AWS credentials provider obtained via aws.NewCredentialsProvider.
// The gitCreds are SigV4-signed Git credentials obtained via authutils.GetGitCredentials
// and are used for go-git ls-remote operations.
func NewAWSCodeCommitProvider(opts Options, credsProvider awssdk.CredentialsProvider, region string, gitCreds *auth.GitCredentials) (*AWSCodeCommitProvider, error) {
	_, parsedRegion, repo, err := parseAWSCodeCommitURL(opts.URL)
	if err != nil {
		return nil, err
	}

	if region == "" {
		region = parsedRegion
	}

	provider := &AWSCodeCommitProvider{
		Region:   region,
		RepoName: repo,
		RepoURL:  opts.URL,
	}

	if credsProvider != nil {
		provider.Client = codecommit.New(codecommit.Options{
			Region:      region,
			Credentials: awssdk.NewCredentialsCache(credsProvider),
		})
	}

	if gitCreds != nil {
		provider.Auth = &githttp.BasicAuth{
			Username: gitCreds.Username,
			Password: gitCreds.Password,
		}
		provider.Remote = gogit.NewRemote(memory.NewStorage(), &gogitconfig.RemoteConfig{
			Name: "origin",
			URLs: []string{opts.URL},
		})
	}

	return provider, nil
}

// ListBranches returns a list of branches from the AWSCodeCommit repository.
// It uses go-git's remote.List() to perform a lightweight ls-remote operation
// with SigV4-signed Git credentials, avoiding per-branch API calls.
func (p *AWSCodeCommitProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	refs, err := p.Remote.ListContext(ctx, &gogit.ListOptions{
		Auth: p.Auth,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list branches: %w", err)
	}

	return parseGoGitBranches(refs, opts.Filters), nil
}

// ListTags returns a list of Git tags from the AWSCodeCommit repository.
// It uses go-git's remote.List() to perform a lightweight ls-remote operation
// with SigV4-signed Git credentials.
func (p *AWSCodeCommitProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	refs, err := p.Remote.ListContext(ctx, &gogit.ListOptions{
		Auth: p.Auth,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list tags: %w", err)
	}

	return parseGoGitTags(refs, opts.Filters), nil
}

// parseGoGitTags extracts tags and their underlying commit SHAs from a slice of go-git references.
func parseGoGitTags(refs []*plumbing.Reference, filters filtering.Filters) []Result {
	// Collect tag names and their SHAs.
	tagMap := make(map[string]string)
	peeledMap := make(map[string]string)
	tags := make([]string, 0)
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()

			// If it's a peeled tag (ends with ^{}), it means it's an annotated tag
			// and this ref points to the actual commit.
			if strings.HasSuffix(tagName, "^{}") {
				tagName = strings.TrimSuffix(tagName, "^{}")
				peeledMap[tagName] = ref.Hash().String()
			} else {
				tagMap[tagName] = ref.Hash().String()
				tags = append(tags, tagName)
			}
		}
	}

	// Override lightweight tag hashes with peeled annotated tag commit hashes where available.
	maps.Copy(tagMap, peeledMap)

	// Apply tag filters (semver, include/exclude regex).
	results := make([]Result, 0, len(tags))
	for _, tagName := range filters.Tags(tags) {
		results = append(results, Result{
			ID:  inputs.ID(tagName),
			SHA: tagMap[tagName],
			Tag: tagName,
		})
	}

	return results
}

// parseGoGitBranches extracts branches and their commit SHAs from a slice of go-git references.
func parseGoGitBranches(refs []*plumbing.Reference, filters filtering.Filters) []Result {
	results := make([]Result, 0)
	for _, ref := range refs {
		if !ref.Name().IsBranch() {
			continue
		}
		branchName := ref.Name().Short()

		if !filters.MatchString(branchName) {
			continue
		}

		results = append(results, Result{
			ID:     inputs.ID(branchName),
			SHA:    ref.Hash().String(),
			Branch: branchName,
		})

		if filters.Limit > 0 && len(results) >= filters.Limit {
			return results
		}
	}

	return results
}

// ListRequests returns a list of open pull requests from the AWSCodeCommit repository.
func (p *AWSCodeCommitProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	var results []Result
	var nextToken *string

	for {
		out, err := p.Client.ListPullRequests(ctx, &codecommit.ListPullRequestsInput{
			RepositoryName:    &p.RepoName,
			PullRequestStatus: cctypes.PullRequestStatusEnumOpen,
			NextToken:         nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %w", err)
		}

		for _, prID := range out.PullRequestIds {
			prOut, err := p.Client.GetPullRequest(ctx, &codecommit.GetPullRequestInput{
				PullRequestId: &prID,
			})
			if err != nil {
				return nil, fmt.Errorf("could not get pull request %q: %w", prID, err)
			}

			pr := prOut.PullRequest
			if pr == nil || len(pr.PullRequestTargets) == 0 {
				continue
			}

			target := pr.PullRequestTargets[0]
			if target.SourceReference == nil || target.SourceCommit == nil {
				continue
			}

			sourceBranch := strings.TrimPrefix(*target.SourceReference, "refs/heads/")

			if !opts.Filters.MatchString(sourceBranch) {
				continue
			}

			var author string
			if pr.AuthorArn != nil {
				// Extract the username from the ARN (last part after /).
				parts := strings.Split(*pr.AuthorArn, "/")
				author = parts[len(parts)-1]
			}

			var title string
			if pr.Title != nil {
				title = *pr.Title
			}

			results = append(results, Result{
				ID:     prID,
				SHA:    *target.SourceCommit,
				Branch: sourceBranch,
				Title:  title,
				Author: author,
			})

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
				return results, nil
			}
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return results, nil
}

// ListEnvironments returns an error as environments are not supported by AWSCodeCommit.
func (p *AWSCodeCommitProvider) ListEnvironments(_ context.Context, _ Options) ([]Result, error) {
	return nil, errors.New("environments not supported by AWSCodeCommit provider")
}

// parseAWSCodeCommitURL parses a AWSCodeCommit URL and returns the host, region, and repo name.
// A AWSCodeCommit URL has the following structure:
// https://git-codecommit.{region}.amazonaws.com/v1/repos/{repository}
func parseAWSCodeCommitURL(ccURL string) (string, string, string, error) {
	u, err := url.Parse(ccURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", ccURL, err)
	}

	if u.Scheme != "https" {
		return "", "", "", fmt.Errorf("invalid AWSCodeCommit URL %q: scheme must be https", ccURL)
	}

	hostParts := strings.Split(u.Hostname(), ".")
	if len(hostParts) < 4 ||
		(!strings.HasPrefix(u.Hostname(), "git-codecommit.") && !strings.HasPrefix(u.Hostname(), "git-codecommit-fips.")) {
		return "", "", "", fmt.Errorf("invalid AWSCodeCommit URL %q: host must start with 'git-codecommit.'", ccURL)
	}

	region := hostParts[1]

	pathParts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "v1" || pathParts[1] != "repos" || pathParts[2] == "" {
		return "", "", "", fmt.Errorf("invalid AWSCodeCommit URL %q: path must be /v1/repos/{repository}", ccURL)
	}

	repo := pathParts[2]

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), region, repo, nil
}

// ParseAWSCodeCommitRegion extracts the AWS region from a AWSCodeCommit URL.
func ParseAWSCodeCommitRegion(urlStr string) (string, error) {
	_, region, _, err := parseAWSCodeCommitURL(urlStr)
	return region, err
}
