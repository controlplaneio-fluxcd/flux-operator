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

type AzureDevOpsProvider struct {
	Client  git.Client
	Owner   string
	Project string
	Repo    string
}

func NewAzureDevOpsProvider(ctx context.Context, opts Options) (*AzureDevOpsProvider, error) {
	var client git.Client

	host, owner, project, repo, err := parseAzureDevOpsURL(opts.URL)
	if err != nil {
		return nil, err
	}

	if host == "https://dev.azure.com" {
		// Create a Azure DevOps connection to your organization
		connection := azuredevops.NewPatConnection(fmt.Sprintf("%s/%s/", host, owner), opts.Token)

		// Create a Azure DevOps client to interact with the git area
		client, err = git.NewClient(ctx, connection)
		if err != nil {
			return nil, fmt.Errorf("could not create Azure DevOps git client: %w", err)
		}
	} else {
		return nil, fmt.Errorf("unsupported Azure DevOps host: %s", host)
	}

	return &AzureDevOpsProvider{
		Client:  client,
		Owner:   owner,
		Project: project,
		Repo:    repo,
	}, nil
}

func (p *AzureDevOpsProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	azRefArguments := git.GetRefsArgs{
		RepositoryId: &p.Repo,
		Project:      &p.Project,
		Filter:       new("tags"),
	}

	// No straightforward api call to list all tags (GetAnnotatedTag() does not return lightweight tags)
	// will filter on "tags" with GetRefs
	azRefs, err := p.Client.GetRefs(ctx, azRefArguments)
	if err != nil {
		return nil, fmt.Errorf("could not list tags: %v", err)
	}

	tagMap := make(map[string]string)
	tags := make([]string, 0, len(azRefs.Value))

	for _, gitRef := range azRefs.Value {
		if gitRef.Name == nil || gitRef.ObjectId == nil {
			continue // skip invalid refs
		}
		tagName := strings.TrimPrefix(*gitRef.Name, "refs/tags/")
		tagMap[tagName] = *gitRef.ObjectId
		tags = append(tags, tagName)
	}

	results := make([]Result, 0)
	for _, tagName := range opts.Filters.Tags(tags) {
		objectID := tagMap[tagName]
		sha := objectID // fallback for lightweight tag

		azTagArguments := git.GetAnnotatedTagArgs{
			Project:      &p.Project,
			RepositoryId: &p.Repo,
			ObjectId:     &objectID,
		}

		// if the tag is annotated, the commit sha is not stored in objectId but a separate api call must be made to access the commit sha
		annotatedTag, err := p.Client.GetAnnotatedTag(ctx, azTagArguments)
		if err == nil && annotatedTag != nil && annotatedTag.ObjectId != nil && annotatedTag.TaggedObject != nil && annotatedTag.TaggedObject.ObjectId != nil {
			sha = *annotatedTag.TaggedObject.ObjectId
		}

		results = append(results, Result{
			ID:  inputs.ID(tagName),
			SHA: sha,
			Tag: tagName,
		})
	}

	return results, nil
}

func (p *AzureDevOpsProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	azBranchArguments := git.GetBranchesArgs{
		RepositoryId: &p.Repo,
		Project:      &p.Project,
	}

	azGitBranches, err := p.Client.GetBranches(ctx, azBranchArguments)
	if err != nil {
		return nil, fmt.Errorf("could not get branches: %v", err)
	}

	results := make([]Result, 0)
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

		if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
			return results, nil
		}
	}

	return results, nil
}

func (p *AzureDevOpsProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	azGitPullRequestsArguments := git.GetPullRequestsArgs{
		RepositoryId:   &p.Repo,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
		Project:        &p.Project,
	}

	azGitPullRequests, err := p.Client.GetPullRequests(ctx, azGitPullRequestsArguments)
	if err != nil {
		return nil, fmt.Errorf("could not list pull requests: %v", err)
	}

	results := make([]Result, 0)
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

		if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
			return results, nil
		}
	}
	return results, nil
}

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
