// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	git "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// AzureDevOpsProvider implements Interface for an Azure DevOps Git repository.
type AzureDevOpsProvider struct {
	Client  azureDevOpsClient
	Owner   string
	Project string
	Repo    string
}

// azureDevOpsClient defines the Azure DevOps Git operations used by the provider.
type azureDevOpsClient interface {
	GetRefs(context.Context, git.GetRefsArgs) (*git.GetRefsResponseValue, error)
	GetAnnotatedTag(context.Context, git.GetAnnotatedTagArgs) (*git.GitAnnotatedTag, error)
	GetBranches(context.Context, git.GetBranchesArgs) (*[]git.GitBranchStats, error)
	GetPullRequests(context.Context, git.GetPullRequestsArgs) (*[]git.GitPullRequest, error)
}

// NewAzureDevOpsProvider creates an Azure DevOps provider from the given options.
func NewAzureDevOpsProvider(_ context.Context, opts Options) (*AzureDevOpsProvider, error) {
	host, owner, project, repo, err := parseAzureDevOpsURL(opts.URL)
	if err != nil {
		return nil, err
	}

	if host == "https://dev.azure.com" {
		// Create a Azure DevOps connection to your organization
		connection := azuredevops.NewPatConnection(fmt.Sprintf("%s/%s/", host, owner), opts.Token)
		client := azuredevops.NewClientWithOptions(connection, connection.BaseUrl,
			azuredevops.WithHTTPClient(newGitProviderHTTPClient(opts.TLSConfig)))
		return &AzureDevOpsProvider{
			Client:  &git.ClientImpl{Client: *client},
			Owner:   owner,
			Project: project,
			Repo:    repo,
		}, nil
	} else {
		return nil, fmt.Errorf("unsupported Azure DevOps host: %s", host)
	}
}

// ListTags returns filtered tags from the Azure DevOps repository.
func (p *AzureDevOpsProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	selector, err := newTagSelector(opts.Filters)
	if err != nil {
		return nil, err
	}
	top := gitProviderPageSize
	azRefArguments := git.GetRefsArgs{
		RepositoryId: &p.Repo,
		Project:      &p.Project,
		Filter:       new("tags"),
		Top:          &top,
	}
	guard := newGitProviderPaginationTokenGuard("Azure DevOps tags")
	token := ""

	// No straightforward api call to list all tags (GetAnnotatedTag() does not return lightweight tags)
	// will filter on "tags" with GetRefs
	for {
		if err := guard.Visit(token); err != nil {
			return nil, err
		}
		if token == "" {
			azRefArguments.ContinuationToken = nil
		} else {
			azRefArguments.ContinuationToken = &token
		}
		azRefs, err := p.Client.GetRefs(ctx, azRefArguments)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		for _, gitRef := range azRefs.Value {
			if gitRef.Name == nil || gitRef.ObjectId == nil {
				continue
			}
			tagName := strings.TrimPrefix(*gitRef.Name, "refs/tags/")
			selector.Add(tagName, Result{
				ID:  inputs.ID(tagName),
				SHA: *gitRef.ObjectId,
				Tag: tagName,
			})
		}
		if azRefs.ContinuationToken == "" {
			break
		}
		token = azRefs.ContinuationToken
	}

	results := selector.Results()
	for i := range results {
		objectID := results[i].SHA
		azTagArguments := git.GetAnnotatedTagArgs{
			Project:      &p.Project,
			RepositoryId: &p.Repo,
			ObjectId:     &objectID,
		}

		// if the tag is annotated, the commit sha is not stored in objectId but a separate api call must be made to access the commit sha
		annotatedTag, err := p.Client.GetAnnotatedTag(ctx, azTagArguments)
		if err == nil && annotatedTag != nil && annotatedTag.ObjectId != nil && annotatedTag.TaggedObject != nil && annotatedTag.TaggedObject.ObjectId != nil {
			results[i].SHA = *annotatedTag.TaggedObject.ObjectId
		}
	}
	return results, nil
}

// ListBranches returns filtered branches from the Azure DevOps repository.
func (p *AzureDevOpsProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	azBranchArguments := git.GetBranchesArgs{
		RepositoryId: &p.Repo,
		Project:      &p.Project,
	}

	azGitBranches, err := p.Client.GetBranches(ctx, azBranchArguments)
	if err != nil {
		return nil, fmt.Errorf("could not get branches: %v", err)
	}

	results := make([]Result, 0, resultLimit)
	for _, branch := range *azGitBranches {
		if branch.Commit == nil || branch.Commit.CommitId == nil || branch.Name == nil {
			continue
		}

		if !opts.Filters.MatchString(*branch.Name) {
			continue
		}

		results = append(results, Result{
			ID:     inputs.ID(*branch.Name),
			SHA:    *branch.Commit.CommitId,
			Branch: *branch.Name,
		})

		if len(results) >= resultLimit {
			return results, nil
		}
	}

	return results, nil
}

// ListRequests returns filtered pull requests from the Azure DevOps repository.
func (p *AzureDevOpsProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	top := gitProviderPageSize
	skip := 0
	azGitPullRequestsArguments := git.GetPullRequestsArgs{
		RepositoryId:   &p.Repo,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
		Project:        &p.Project,
		Top:            &top,
		Skip:           &skip,
	}
	guard := newGitProviderPaginationGuard("Azure DevOps pull requests")

	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(skip); err != nil {
			return nil, err
		}
		azGitPullRequestsArguments.Skip = &skip
		azGitPullRequests, err := p.Client.GetPullRequests(ctx, azGitPullRequestsArguments)
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %v", err)
		}
		for _, pr := range *azGitPullRequests {
			if pr.SourceRefName == nil || pr.PullRequestId == nil || pr.LastMergeSourceCommit == nil || pr.LastMergeSourceCommit.CommitId == nil || pr.Title == nil || pr.CreatedBy == nil || pr.CreatedBy.DisplayName == nil {
				continue
			}

			sourceBranch := strings.TrimPrefix(*pr.SourceRefName, "refs/heads/")
			if !opts.Filters.MatchString(sourceBranch) {
				continue
			}

			prLabels := []string{}
			if pr.Labels != nil {
				prLabels = make([]string, len(*pr.Labels))
				for i, l := range *pr.Labels {
					prLabels[i] = *l.Name
				}
			}
			if !opts.Filters.MatchLabels(prLabels) {
				continue
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", *pr.PullRequestId),
				SHA:    *pr.LastMergeSourceCommit.CommitId,
				Branch: sourceBranch,
				Title:  *pr.Title,
				Author: *pr.CreatedBy.DisplayName,
				Labels: prLabels,
			})
			if len(results) >= resultLimit {
				return results, nil
			}
		}
		if len(*azGitPullRequests) < top {
			break
		}
		skip += len(*azGitPullRequests)
	}
	return results, nil
}

// ListEnvironments reports that Azure DevOps environments are unsupported.
func (p *AzureDevOpsProvider) ListEnvironments(ctx context.Context, opts Options) ([]Result, error) {
	return nil, errors.New("environments not supported by Azure DevOps provider")
}

// parseAzureDevOpsURL parses a AzureDevOps URL and returns the host, owner, project and repo.
// a AzureDevOps URL has the following structure: https://dev.azure.com/{organization}/{project}/_git/{repository}
func parseAzureDevOpsURL(azURL string) (string, string, string, string, error) {
	u, err := url.Parse(azURL)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid URL %q: %w", azURL, err)
	}

	parts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "_git" {
		return "", "", "", "", fmt.Errorf("invalid AzureDevOps URL %q: can't find owner and repository", azURL)
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), parts[0], parts[1], parts[3], nil
}
