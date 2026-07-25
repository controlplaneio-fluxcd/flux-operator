// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// GitLabProvider implements Interface for a GitLab project.
type GitLabProvider struct {
	Client  *gitlab.Client
	Project string
}

// NewGitLabProvider creates a GitLab provider from the given options.
func NewGitLabProvider(ctx context.Context, opts Options) (*GitLabProvider, error) {
	var client *gitlab.Client
	var glOpts []gitlab.ClientOptionFunc

	host, project, err := parseGitLabURL(opts.URL)
	if err != nil {
		return nil, err
	}

	rtClient := retryablehttp.NewClient()
	rtClient.HTTPClient = newGitProviderHTTPClient(opts.TLSConfig)
	glOpts = append(glOpts, gitlab.WithHTTPClient(rtClient.HTTPClient))

	if host != "https://gitlab.com" {
		glOpts = append(glOpts, gitlab.WithBaseURL(host))
	}

	client, err = gitlab.NewClient(opts.Token, glOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create GitLab client: %v", err)
	}

	return &GitLabProvider{
		Client:  client,
		Project: project,
	}, nil
}

// ListTags returns filtered tags from the GitLab project.
func (p *GitLabProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	selector, err := newTagSelector(opts.Filters)
	if err != nil {
		return nil, err
	}
	glOpts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{PerPage: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("GitLab tags")

	for {
		if err := guard.Visit(int(glOpts.Page)); err != nil {
			return nil, err
		}
		page, resp, err := p.Client.Tags.ListTags(p.Project, glOpts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		for _, tag := range page {
			selector.Add(tag.Name, Result{
				ID:   inputs.ID(tag.Name),
				SHA:  tag.Commit.ID,
				Tag:  tag.Name,
				Slug: gitlabSlugify(tag.Name),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		glOpts.Page = resp.NextPage
	}
	return selector.Results(), nil
}

// ListBranches returns filtered branches from the GitLab project.
func (p *GitLabProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	glOpts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{PerPage: gitProviderPageSize},
	}
	if opts.Filters.Include != nil {
		glOpts.Regex = new(opts.Filters.Include.String())
	}

	guard := newGitProviderPaginationGuard("GitLab branches")
	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(int(glOpts.Page)); err != nil {
			return nil, err
		}
		branches, resp, err := p.Client.Branches.ListBranches(p.Project, glOpts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("could not list branches: %v", err)
		}

		for _, branch := range branches {
			if !opts.Filters.MatchString(branch.Name) {
				continue
			}

			results = append(results, Result{
				ID:     inputs.ID(branch.Name),
				SHA:    branch.Commit.ID,
				Branch: branch.Name,
				Slug:   gitlabSlugify(branch.Name),
			})

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		glOpts.Page = resp.NextPage
	}

	return results, nil
}

// ListRequests returns filtered merge requests from the GitLab project.
func (p *GitLabProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	var labels *gitlab.LabelOptions
	if len(opts.Filters.Labels) > 0 {
		var lo gitlab.LabelOptions = opts.Filters.Labels
		labels = &lo
	}

	glOpts := &gitlab.ListProjectMergeRequestsOptions{
		State:       new("opened"),
		Labels:      labels,
		ListOptions: gitlab.ListOptions{PerPage: gitProviderPageSize},
	}

	guard := newGitProviderPaginationGuard("GitLab merge requests")
	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(int(glOpts.Page)); err != nil {
			return nil, err
		}
		msrs, resp, err := p.Client.MergeRequests.ListProjectMergeRequests(p.Project, glOpts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("could not list merge requests: %v", err)
		}

		for _, mr := range msrs {
			if !opts.Filters.MatchString(mr.SourceBranch) {
				continue
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", mr.IID),
				SHA:    mr.SHA,
				Branch: mr.SourceBranch,
				Slug:   gitlabSlugify(mr.SourceBranch),
				Title:  mr.Title,
				Author: mr.Author.Username,
				Labels: mr.Labels,
			})

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		glOpts.Page = resp.NextPage
	}

	return results, nil
}

// ListEnvironments returns filtered deployment environments from the GitLab project.
func (p *GitLabProvider) ListEnvironments(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	glOpts := &gitlab.ListEnvironmentsOptions{
		ListOptions: gitlab.ListOptions{PerPage: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("GitLab environments")

	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(int(glOpts.Page)); err != nil {
			return nil, err
		}
		envs, resp, err := p.Client.Environments.ListEnvironments(p.Project, glOpts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("could not list environments: %v", err)
		}

		for _, env := range envs {
			if !opts.Filters.MatchString(env.Name) {
				continue
			}

			// We need to also consider "running" deployments to allow users to `flux-operator reconcile rsip ...` in the deployment job itself.
			// This is only available through the Deployments API.
			deployments, _, err := p.Client.Deployments.ListProjectDeployments(p.Project, &gitlab.ListProjectDeploymentsOptions{
				ListOptions: gitlab.ListOptions{},
				OrderBy:     new("created_at"),
				Sort:        new("desc"),
				Environment: new(env.Name),
			}, gitlab.WithContext(ctx))
			if err != nil {
				return nil, fmt.Errorf(`could not list deployments for environment "%s": %v`, env.Name, err)
			}

			var lastDeployment *gitlab.Deployment
			for _, deployment := range deployments {
				// When an environment has been stopped, it will stay so until the next deployment job has finished successfully.
				// There still will be a new running deployment during this time, however, so we can filter for that.
				// When the environment is available (again), also consider the latest successful deployment.
				if deployment.Status == "running" || (env.State == "available" && deployment.Status == "success") {
					lastDeployment = deployment
					break
				}
			}

			if lastDeployment == nil {
				continue
			}

			author := ""
			if lastDeployment.User != nil {
				author = lastDeployment.User.Username
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", env.ID),
				SHA:    lastDeployment.Deployable.Commit.ID,
				Branch: lastDeployment.Deployable.Ref,
				Title:  env.Name,
				Slug:   env.Slug,
				Author: author,
			})

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		glOpts.Page = resp.NextPage
	}

	return results, nil
}

// parseGitLabURL parses a GitLab URL and returns the host and project.
func parseGitLabURL(glURL string) (string, string, error) {
	u, err := url.Parse(glURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", glURL, err)
	}

	project := strings.TrimLeft(u.Path, "/")
	if len(project) < 1 {
		return "", "", fmt.Errorf("invalid GitLab URL %q: can't find project", glURL)
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), project, nil
}

const gitLabSlugMaxLength = 63

var nonGitLabSlugCharactersRegexp = regexp.MustCompile(`[^a-z0-9-]`)

// gitlabSlugify matches GitLab's slugification scheme, cf. https://gitlab.com/gitlab-org/gitlab/-/blob/0fd5cad2e2a2dc8ccc4ba359c4fdcdcf7a38ace8/gems/gitlab-utils/lib/gitlab/utils.rb#L65
// gitlabSlugify converts a GitLab ref into its CI_COMMIT_REF_SLUG form.
func gitlabSlugify(value string) string {
	value = strings.ToLower(value)
	value = nonGitLabSlugCharactersRegexp.ReplaceAllString(value, "-")
	if len(value) > gitLabSlugMaxLength {
		value = value[:gitLabSlugMaxLength]
	}
	return strings.Trim(value, "-")
}
