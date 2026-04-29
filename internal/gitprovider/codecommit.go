// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	cctypes "github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/fluxcd/pkg/auth"
	fluxaws "github.com/fluxcd/pkg/auth/aws"
	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// CodeCommitProvider implements the gitprovider.Interface for AWS CodeCommit.
type CodeCommitProvider struct {
	Client   *codecommit.Client
	Token    auth.Token
	Region   string
	RepoName string
	RepoURL  string
}

// NewCodeCommitProvider creates a new CodeCommit provider from the given options.
// The token must be an *aws.Credentials obtained via auth.GetAccessToken.
func NewCodeCommitProvider(_ context.Context, opts Options, token auth.Token) (*CodeCommitProvider, error) {
	_, region, repo, err := parseCodeCommitURL(opts.URL)
	if err != nil {
		return nil, err
	}

	awsCreds, ok := token.(*fluxaws.Credentials)
	if !ok {
		return nil, fmt.Errorf("failed to cast token to AWS credentials: %T", token)
	}

	client := codecommit.New(codecommit.Options{
		Region: region,
		Credentials: aws.NewCredentialsCache(
			aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     *awsCreds.AccessKeyId,
					SecretAccessKey: *awsCreds.SecretAccessKey,
					SessionToken:    *awsCreds.SessionToken,
					Expires:         *awsCreds.Expiration,
					CanExpire:       true,
				}, nil
			}),
		),
	})

	return &CodeCommitProvider{
		Client:   client,
		Token:    token,
		Region:   region,
		RepoName: repo,
		RepoURL:  opts.URL,
	}, nil
}

// ListBranches returns a list of branches from the CodeCommit repository.
func (p *CodeCommitProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	var results []Result
	var nextToken *string

	for {
		out, err := p.Client.ListBranches(ctx, &codecommit.ListBranchesInput{
			RepositoryName: &p.RepoName,
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("could not list branches: %w", err)
		}

		for _, branchName := range out.Branches {
			if !opts.Filters.MatchString(branchName) {
				continue
			}

			branchOut, err := p.Client.GetBranch(ctx, &codecommit.GetBranchInput{
				RepositoryName: &p.RepoName,
				BranchName:     &branchName,
			})
			if err != nil {
				return nil, fmt.Errorf("could not get branch %q: %w", branchName, err)
			}
			if branchOut.Branch == nil || branchOut.Branch.CommitId == nil {
				continue
			}

			results = append(results, Result{
				ID:     inputs.ID(branchName),
				SHA:    *branchOut.Branch.CommitId,
				Branch: branchName,
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

// ListTags returns a list of Git tags from the CodeCommit repository.
// Since CodeCommit does not have an API for listing Git tags, this method
// uses go-git's remote.List() to perform a lightweight ls-remote operation
// with SigV4-signed Git credentials.
func (p *CodeCommitProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	gitURL, err := url.Parse(p.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CodeCommit URL: %w", err)
	}

	// Generate SigV4 Git credentials from the AWS access token.
	provider := fluxaws.Provider{}
	username, password, err := provider.NewCodeCommitGitCredentials(
		ctx,
		[]auth.Token{p.Token},
		auth.WithGitURL(*gitURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CodeCommit Git credentials: %w", err)
	}

	remote := gogit.NewRemote(memory.NewStorage(), &gogitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{p.RepoURL},
	})

	refs, err := remote.ListContext(ctx, &gogit.ListOptions{
		Auth: &githttp.BasicAuth{
			Username: username,
			Password: password,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not list tags: %w", err)
	}

	// Collect tag names and their SHAs.
	tagMap := make(map[string]string)
	tags := make([]string, 0)
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()
			tagMap[tagName] = ref.Hash().String()
			tags = append(tags, tagName)
		}
	}

	// Apply tag filters (semver, include/exclude regex).
	results := make([]Result, 0)
	for _, tagName := range opts.Filters.Tags(tags) {
		results = append(results, Result{
			ID:  inputs.ID(tagName),
			SHA: tagMap[tagName],
			Tag: tagName,
		})
	}

	return results, nil
}

// ListRequests returns a list of open pull requests from the CodeCommit repository.
func (p *CodeCommitProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
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

// ListEnvironments returns an error as environments are not supported by CodeCommit.
func (p *CodeCommitProvider) ListEnvironments(_ context.Context, _ Options) ([]Result, error) {
	return nil, errors.New("environments not supported by CodeCommit provider")
}

// parseCodeCommitURL parses a CodeCommit URL and returns the host, region, and repo name.
// A CodeCommit URL has the following structure:
// https://git-codecommit.{region}.amazonaws.com/v1/repos/{repository}
func parseCodeCommitURL(ccURL string) (string, string, string, error) {
	u, err := url.Parse(ccURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", ccURL, err)
	}

	if u.Scheme != "https" {
		return "", "", "", fmt.Errorf("invalid CodeCommit URL %q: scheme must be https", ccURL)
	}

	hostParts := strings.Split(u.Hostname(), ".")
	if len(hostParts) < 4 ||
		(!strings.HasPrefix(u.Hostname(), "git-codecommit.") && !strings.HasPrefix(u.Hostname(), "git-codecommit-fips.")) {
		return "", "", "", fmt.Errorf("invalid CodeCommit URL %q: host must start with 'git-codecommit.'", ccURL)
	}

	region := hostParts[1]

	pathParts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(pathParts) != 3 || pathParts[0] != "v1" || pathParts[1] != "repos" || pathParts[2] == "" {
		return "", "", "", fmt.Errorf("invalid CodeCommit URL %q: path must be /v1/repos/{repository}", ccURL)
	}

	repo := pathParts[2]

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), region, repo, nil
}

// ParseCodeCommitRegion extracts the AWS region from a CodeCommit URL.
func ParseCodeCommitRegion(urlStr string) (string, error) {
	_, region, _, err := parseCodeCommitURL(urlStr)
	return region, err
}
