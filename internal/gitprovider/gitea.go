// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"code.gitea.io/sdk/gitea"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// GiteaProvider implements Interface for a Gitea or Forgejo repository.
type GiteaProvider struct {
	Client *gitea.Client
	Owner  string
	Repo   string
	mu     sync.Mutex
}

// NewGiteaProvider creates a Gitea provider from the given options.
func NewGiteaProvider(ctx context.Context, opts Options) (*GiteaProvider, error) {
	host, owner, repo, err := parseGiteaURL(opts.URL)
	if err != nil {
		return nil, err
	}

	clientOpts := []gitea.ClientOption{
		gitea.SetContext(ctx),
		gitea.SetHTTPClient(newGitProviderHTTPClient(opts.TLSConfig)),
	}

	if opts.Token != "" {
		clientOpts = append(clientOpts, gitea.SetToken(opts.Token))
	}

	client, err := gitea.NewClient(host, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create Gitea client: %v", err)
	}

	return &GiteaProvider{
		Client: client,
		Owner:  owner,
		Repo:   repo,
	}, nil
}

// ListTags returns filtered tags from the Gitea repository.
func (p *GiteaProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Client.SetContext(ctx)
	selector, err := newTagSelector(opts.Filters)
	if err != nil {
		return nil, err
	}
	giteaOpts := gitea.ListRepoTagsOptions{
		ListOptions: gitea.ListOptions{PageSize: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("Gitea tags")

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := guard.Visit(giteaOpts.Page); err != nil {
			return nil, err
		}
		page, resp, err := p.Client.ListRepoTags(p.Owner, p.Repo, giteaOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		for _, tag := range page {
			selector.Add(tag.Name, Result{
				ID:  inputs.ID(tag.Name),
				SHA: tag.Commit.SHA,
				Tag: tag.Name,
			})
		}

		if resp.NextPage == 0 {
			break
		}
		giteaOpts.Page = resp.NextPage
	}
	return selector.Results(), nil
}

// ListBranches returns filtered branches from the Gitea repository.
func (p *GiteaProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Client.SetContext(ctx)
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	giteaOpts := gitea.ListRepoBranchesOptions{
		ListOptions: gitea.ListOptions{PageSize: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("Gitea branches")

	results := make([]Result, 0, resultLimit)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := guard.Visit(giteaOpts.Page); err != nil {
			return nil, err
		}
		branches, resp, err := p.Client.ListRepoBranches(p.Owner, p.Repo, giteaOpts)
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
			})

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		giteaOpts.Page = resp.NextPage
	}

	return results, nil
}

// ListRequests returns filtered pull requests from the Gitea repository.
func (p *GiteaProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Client.SetContext(ctx)
	resultLimit, err := gitProviderResultLimit(opts.Filters)
	if err != nil {
		return nil, err
	}
	giteaOpts := gitea.ListPullRequestsOptions{
		State:       gitea.StateOpen,
		ListOptions: gitea.ListOptions{PageSize: gitProviderPageSize},
	}
	guard := newGitProviderPaginationGuard("Gitea pull requests")

	results := make([]Result, 0, resultLimit)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := guard.Visit(giteaOpts.Page); err != nil {
			return nil, err
		}
		prs, resp, err := p.Client.ListRepoPullRequests(p.Owner, p.Repo, giteaOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %v", err)
		}

		for _, pr := range prs {
			if !opts.Filters.MatchString(pr.Head.Ref) {
				continue
			}

			prLabels := make([]string, len(pr.Labels))
			for i, l := range pr.Labels {
				prLabels[i] = l.Name
			}

			if !opts.Filters.MatchLabels(prLabels) {
				continue
			}

			results = append(results, Result{
				ID:     fmt.Sprintf("%d", pr.Index),
				SHA:    pr.Head.Sha,
				Branch: pr.Head.Ref,
				Title:  pr.Title,
				Author: pr.Poster.UserName,
				Labels: prLabels,
			})

			if len(results) >= resultLimit {
				return results, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		giteaOpts.Page = resp.NextPage
	}

	return results, nil
}

// ListEnvironments reports that Gitea environments are unsupported.
func (p *GiteaProvider) ListEnvironments(ctx context.Context, opts Options) ([]Result, error) {
	return nil, errors.New("environments not supported by Gitea provider")
}

// parseGiteaURL parses a Gitea URL and returns the host, owner, and repo.
func parseGiteaURL(giteaURL string) (string, string, string, error) {
	u, err := url.Parse(giteaURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL %q: %w", giteaURL, err)
	}

	parts := strings.Split(strings.TrimLeft(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid Gitea URL %q: can't find owner and repository", giteaURL)
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), parts[0], parts[1], nil
}
