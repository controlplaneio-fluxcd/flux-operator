// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-github/v87/github"
	"golang.org/x/oauth2"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// GitHubProvider implements Interface for a GitHub repository.
type GitHubProvider struct {
	Client *github.Client
	Owner  string
	Repo   string
}

// NewGitHubProvider creates a GitHub provider from the given options.
func NewGitHubProvider(ctx context.Context, opts Options) (*GitHubProvider, error) {
	var ts oauth2.TokenSource

	if opts.Token != "" {
		ts = oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
	}

	host, owner, repo, err := parseGitHubURL(opts.URL)
	if err != nil {
		return nil, err
	}

	httpClient := newGitProviderHTTPClient(opts.TLSConfig)
	httpCtx := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	clientOpts := []github.ClientOptionsFunc{
		github.WithHTTPClient(oauth2.NewClient(httpCtx, ts)),
	}
	if host != "https://github.com" {
		clientOpts = append(clientOpts, github.WithEnterpriseURLs(host, host))
	}

	client, err := github.NewClient(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create GitHub client: %w", err)
	}

	return &GitHubProvider{
		Client: client,
		Owner:  owner,
		Repo:   repo,
	}, nil
}

// ListTags returns filtered tags from the GitHub repository.
func (p *GitHubProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	selector, err := newTagSelector(opts.Filters)
	if err != nil {
		return nil, err
	}
	ghOpts := &github.ListOptions{PerPage: gitProviderPageSize}
	guard := newGitProviderPaginationGuard("GitHub tags")

	for {
		if err := guard.Visit(ghOpts.Page); err != nil {
			return nil, err
		}
		page, resp, err := p.Client.Repositories.ListTags(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		for _, tag := range page {
			name := tag.GetName()
			selector.Add(name, Result{
				ID:  inputs.ID(name),
				SHA: tag.GetCommit().GetSHA(),
				Tag: name,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		ghOpts.Page = resp.NextPage
	}
	return selector.Results(), nil
}

// ListBranches returns filtered branches from the GitHub repository.
func (p *GitHubProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	ghOpts := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("GitHub branches")

	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(ghOpts.Page); err != nil {
			return nil, err
		}
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

			if len(results) >= resultLimit {
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

// ListRequests returns filtered pull requests from the GitHub repository.
func (p *GitHubProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	ghOpts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("GitHub pull requests")

	results := make([]Result, 0, resultLimit)
	for {
		if err := guard.Visit(ghOpts.Page); err != nil {
			return nil, err
		}
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

			if len(results) >= resultLimit {
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

// ListEnvironments reports that GitHub environments are unsupported.
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
