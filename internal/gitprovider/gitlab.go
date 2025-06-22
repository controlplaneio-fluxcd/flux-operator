// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
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
	if opts.CertPool != nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: opts.CertPool,
			},
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
			ID:  inputs.ID(tag.Name),
			SHA: tag.Commit.ID,
			Tag: tag.Name,
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
			if !opts.Filters.MatchBranch(branch.Name) {
				continue
			}

			results = append(results, Result{
				ID:     inputs.ID(branch.Name),
				SHA:    branch.Commit.ID,
				Branch: branch.Name,
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
			if !opts.Filters.MatchBranch(mr.SourceBranch) {
				continue
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", mr.IID),
				SHA:    mr.SHA,
				Branch: mr.SourceBranch,
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
