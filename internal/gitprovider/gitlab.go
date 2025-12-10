// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/go-retryablehttp"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

type GitLabProvider struct {
	Client  *gitlab.Client
	Project string
}

func NewGitLabProvider(ctx context.Context, opts Options) (*GitLabProvider, error) {
	var client *gitlab.Client
	var glOpts []gitlab.ClientOptionFunc

	host, project, err := parseGitLabURL(opts.URL)
	if err != nil {
		return nil, err
	}

	rtClient := retryablehttp.NewClient()
	if opts.TLSConfig != nil {
		tr := &http.Transport{
			TLSClientConfig: opts.TLSConfig,
		}
		rtClient.HTTPClient.Transport = tr
	}
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

func (p *GitLabProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	glOpts := &gitlab.ListTagsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	gitlabTags := make([]*gitlab.Tag, 0)
	for {
		page, resp, err := p.Client.Tags.ListTags(p.Project, glOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		gitlabTags = append(gitlabTags, page...)

		if resp.NextPage == 0 {
			break
		}
		glOpts.Page = resp.NextPage
	}

	tagMap := make(map[string]*gitlab.Tag, len(gitlabTags))
	tags := make([]string, 0, len(gitlabTags))
	for _, tag := range gitlabTags {
		tags = append(tags, tag.Name)
		tagMap[tag.Name] = tag
	}

	results := make([]Result, 0)
	for _, version := range opts.Filters.Tags(tags) {
		tag, ok := tagMap[version]
		if !ok {
			return nil, fmt.Errorf("could not find tag %s", version)
		}

		results = append(results, Result{
			ID:   inputs.ID(tag.Name),
			SHA:  tag.Commit.ID,
			Tag:  tag.Name,
			Slug: gitlabSlugify(tag.Name),
		})

		if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
			return results, nil
		}
	}
	return results, nil
}

func (p *GitLabProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	glOpts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}
	if opts.Filters.Include != nil {
		glOpts.Regex = gitlab.Ptr(opts.Filters.Include.String())
	}

	results := make([]Result, 0)
	for {
		branches, resp, err := p.Client.Branches.ListBranches(p.Project, glOpts)
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

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
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

func (p *GitLabProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	var labels *gitlab.LabelOptions
	if len(opts.Filters.Labels) > 0 {
		var lo gitlab.LabelOptions = opts.Filters.Labels
		labels = &lo
	}

	glOpts := &gitlab.ListProjectMergeRequestsOptions{
		State:  gitlab.Ptr("opened"),
		Labels: labels,
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	results := make([]Result, 0)
	for {
		msrs, resp, err := p.Client.MergeRequests.ListProjectMergeRequests(p.Project, glOpts)
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

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
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

func (p *GitLabProvider) ListEnvironments(ctx context.Context, opts Options) ([]Result, error) {
	glOpts := &gitlab.ListEnvironmentsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	results := make([]Result, 0)
	for {
		envs, resp, err := p.Client.Environments.ListEnvironments(p.Project, glOpts)
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
				OrderBy:     gitlab.Ptr("created_at"),
				Sort:        gitlab.Ptr("desc"),
				Environment: gitlab.Ptr(env.Name),
			})
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

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
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

// parseGitHubURL parses a GitLab URL and returns the host and project.
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
func gitlabSlugify(value string) string {
	value = strings.ToLower(value)
	value = nonGitLabSlugCharactersRegexp.ReplaceAllString(value, "-")
	if len(value) > gitLabSlugMaxLength {
		value = value[:gitLabSlugMaxLength]
	}
	return strings.Trim(value, "-")
}
