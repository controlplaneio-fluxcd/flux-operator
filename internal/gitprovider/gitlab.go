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

func (p *GitLabProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	glOpts := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}
	if opts.Filters.SourceBranchRx != nil {
		glOpts.Regex = gitlab.Ptr(opts.Filters.SourceBranchRx.String())
	}

	var results []Result
	for {
		branches, resp, err := p.Client.Branches.ListBranches(p.Project, glOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list merge requests: %v", err)
		}

		for _, branch := range branches {
			results = append(results, Result{
				ID:           checksum(branch.Name),
				SHA:          branch.Commit.ID,
				SourceBranch: branch.Name,
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

	var results []Result
	for {
		msrs, resp, err := p.Client.MergeRequests.ListProjectMergeRequests(p.Project, glOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list merge requests: %v", err)
		}

		for _, mr := range msrs {
			if !matchBranches(opts, mr.SourceBranch, mr.TargetBranch) {
				continue
			}

			results = append(results, Result{
				ID:           fmt.Sprintf("%d", mr.IID),
				SHA:          mr.SHA,
				SourceBranch: mr.SourceBranch,
				TargetBranch: mr.TargetBranch,
				Title:        mr.Title,
				Author:       mr.Author.Username,
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
func parseGitLabURL(ghURL string) (string, string, error) {
	u, err := url.Parse(ghURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", ghURL, err)
	}

	project := strings.TrimLeft(u.Path, "/")
	if len(project) < 1 {
		return "", "", fmt.Errorf("invalid GitLab URL %q: can't find project", ghURL)
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), project, nil
}
