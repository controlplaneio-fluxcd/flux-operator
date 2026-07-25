// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	cctypes "github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/fluxcd/pkg/auth"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// AWSCodeCommitProvider implements the gitprovider.Interface for AWS AWSCodeCommit.
type AWSCodeCommitProvider struct {
	Client   awsCodeCommitClient
	HTTP     *http.Client
	Auth     *githttp.BasicAuth
	Region   string
	RepoName string
	RepoURL  string
}

// awsCodeCommitClient defines the CodeCommit API operations used by the provider.
type awsCodeCommitClient interface {
	ListPullRequests(context.Context, *codecommit.ListPullRequestsInput, ...func(*codecommit.Options)) (*codecommit.ListPullRequestsOutput, error)
	GetPullRequest(context.Context, *codecommit.GetPullRequestInput, ...func(*codecommit.Options)) (*codecommit.GetPullRequestOutput, error)
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
		HTTP:     newGitProviderHTTPClient(opts.TLSConfig),
	}

	if credsProvider != nil {
		provider.Client = codecommit.New(codecommit.Options{
			Region:      region,
			Credentials: awssdk.NewCredentialsCache(credsProvider),
			HTTPClient:  provider.HTTP,
		})
	}

	if gitCreds != nil {
		provider.Auth = &githttp.BasicAuth{
			Username: gitCreds.Username,
			Password: gitCreds.Password,
		}
	}

	return provider, nil
}

// ListBranches returns a list of branches from the AWSCodeCommit repository.
// It performs a lightweight ls-remote operation via listRefs with SigV4-signed
// Git credentials over a response-limited HTTP client, avoiding per-branch API calls.
func (p *AWSCodeCommitProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	if _, err := gitProviderResultLimit(opts.Filters); err != nil {
		return nil, err
	}
	refs, err := p.listRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not list branches: %w", err)
	}

	return parseGoGitBranches(refs, opts.Filters)
}

// ListTags returns a list of Git tags from the AWSCodeCommit repository.
// It performs a lightweight ls-remote operation via listRefs with SigV4-signed
// Git credentials over a response-limited HTTP client.
func (p *AWSCodeCommitProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	if _, err := gitProviderResultLimit(opts.Filters); err != nil {
		return nil, err
	}
	refs, err := p.listRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not list tags: %w", err)
	}

	return parseGoGitTags(refs, opts.Filters)
}

// parseGoGitTags extracts tags and their underlying commit SHAs from a slice of go-git references.
func parseGoGitTags(refs []*plumbing.Reference, filters filtering.Filters) ([]Result, error) {
	selector, err := newTagSelector(filters)
	if err != nil {
		return nil, err
	}

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
	for _, tagName := range tags {
		selector.Add(tagName, Result{
			ID:  inputs.ID(tagName),
			SHA: tagMap[tagName],
			Tag: tagName,
		})
	}

	return selector.Results(), nil
}

// parseGoGitBranches extracts branches and their commit SHAs from a slice of go-git references.
func parseGoGitBranches(refs []*plumbing.Reference, filters filtering.Filters) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(filters)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, resultLimit)
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

		if len(results) >= resultLimit {
			return results, nil
		}
	}

	return results, nil
}

// ListRequests returns a list of open pull requests from the AWSCodeCommit repository.
func (p *AWSCodeCommitProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, resultLimit)
	var nextToken *string
	guard := newGitProviderPaginationTokenGuard("AWS CodeCommit pull requests")

	for {
		token := ""
		if nextToken != nil {
			token = *nextToken
		}
		if err := guard.Visit(token); err != nil {
			return nil, err
		}
		maxResults := int32(gitProviderPageSize)
		out, err := p.Client.ListPullRequests(ctx, &codecommit.ListPullRequestsInput{
			RepositoryName:    &p.RepoName,
			PullRequestStatus: cctypes.PullRequestStatusEnumOpen,
			NextToken:         nextToken,
			MaxResults:        &maxResults,
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

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	return results, nil
}

// listRefs returns remote refs using a response-limited Git smart HTTP client.
func (p *AWSCodeCommitProvider) listRefs(ctx context.Context) (results []*plumbing.Reference, err error) {
	endpoint, err := transport.NewEndpoint(p.RepoURL)
	if err != nil {
		return nil, err
	}
	httpClient := p.HTTP
	if httpClient == nil {
		httpClient = newGitProviderHTTPClient(nil)
	}
	session, err := githttp.NewClient(httpClient).NewUploadPackSession(endpoint, p.Auth)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := session.Close(); err == nil {
			err = closeErr
		}
	}()

	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, err
	}
	allRefs, err := advertised.AllReferences()
	if err != nil {
		return nil, err
	}
	iter, err := allRefs.IterReferences()
	if err != nil {
		return nil, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		results = append(results, ref)
		return nil
	})
	return results, err
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
