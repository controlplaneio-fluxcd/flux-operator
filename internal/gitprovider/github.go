// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

type GitHubProvider struct {
	Client *github.Client
	Owner  string
	Repo   string
}

func NewGitHubProvider(ctx context.Context, opts Options) (*GitHubProvider, error) {
	var client *github.Client
	var ts oauth2.TokenSource

	if opts.Token != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
	}

	host, owner, repo, err := parseGitHubURL(opts.URL)
	if err != nil {
		return nil, err
	}

	if host == "https://github.com" {
		// Create a GitHub client for GitHub.com
		client = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		// Create a GitHub client for GitHub Enterprise with a custom cert pool.
		var httpClient *http.Client
		if opts.TLSConfig != nil {
			tr := &http.Transport{
				TLSClientConfig: opts.TLSConfig,
			}
			ctxCA := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: tr})
			httpClient = oauth2.NewClient(ctxCA, ts)
		} else {
			// Create OAuth2 client without custom cert pool
			httpClient = oauth2.NewClient(ctx, ts)
		}
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(host, host)
		if err != nil {
			return nil, fmt.Errorf("could not create enterprise GitHub client: %v", err)
		}
	}

	return &GitHubProvider{
		Client: client,
		Owner:  owner,
		Repo:   repo,
	}, nil
}

func (p *GitHubProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	ghOpts := &github.ListOptions{
		PerPage: 100,
	}

	repoTags := make([]*github.RepositoryTag, 0)
	for {
		page, resp, err := p.Client.Repositories.ListTags(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		repoTags = append(repoTags, page...)

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	tagMap := make(map[string]*github.RepositoryTag, len(repoTags))
	tags := make([]string, 0, len(repoTags))
	for _, tag := range repoTags {
		tags = append(tags, tag.GetName())
		tagMap[tag.GetName()] = tag
	}

	results := make([]Result, 0)
	for _, version := range opts.Filters.Tags(tags) {
		tag, ok := tagMap[version]
		if !ok {
			return nil, fmt.Errorf("could not find tag %s", version)
		}

		results = append(results, Result{
			ID:  inputs.ID(tag.GetName()),
			SHA: tag.GetCommit().GetSHA(),
			Tag: tag.GetName(),
		})

		if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
			return results, nil
		}
	}
	return results, nil
}

func (p *GitHubProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	ghOpts := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	results := make([]Result, 0)
	for {
		branches, resp, err := p.Client.Repositories.ListBranches(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list branches: %v", err)
		}

		for _, branch := range branches {
			if !opts.Filters.MatchString(branch.GetName()) {
				continue
			}

			results = append(results, Result{
				ID:     inputs.ID(branch.GetName()),
				SHA:    branch.GetCommit().GetSHA(),
				Branch: branch.GetName(),
			})

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return results, nil
}

func (p *GitHubProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	ghOpts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	results := make([]Result, 0)
	for {
		prs, resp, err := p.Client.PullRequests.List(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %v", err)
		}

		for _, pr := range prs {
			if !opts.Filters.MatchString(pr.GetHead().GetRef()) {
				continue
			}

			prLabels := make([]string, len(pr.Labels))
			for i, l := range pr.Labels {
				prLabels[i] = l.GetName()
			}

			if !opts.Filters.MatchLabels(prLabels) {
				continue
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", pr.GetNumber()),
				SHA:    pr.GetHead().GetSHA(),
				Branch: pr.GetHead().GetRef(),
				Title:  pr.GetTitle(),
				Author: pr.GetUser().GetLogin(),
				Labels: prLabels,
			})

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}

	return results, nil
}

func (p *GitHubProvider) ListEnvironments(ctx context.Context, opts Options) ([]Result, error) {
	return nil, errors.New("environments not supported by GitHub provider")
}

// parseGitHubURL parses a GitHub URL and returns the host, owner, and repo.
func parseGitHubURL(ghURL string) (string, string, string, error) {
	u, err := url.Parse(ghURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", ghURL, err)
	}

	parts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid GitHub URL %q: can't find owner and repository", ghURL)
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), parts[0], parts[1], nil
}
