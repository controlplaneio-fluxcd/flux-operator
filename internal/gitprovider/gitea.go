// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/sdk/gitea"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

type GiteaProvider struct {
	Client *gitea.Client
	Owner  string
	Repo   string
}

func NewGiteaProvider(ctx context.Context, opts Options) (*GiteaProvider, error) {
	host, owner, repo, err := parseGiteaURL(opts.URL)
	if err != nil {
		return nil, err
	}

	clientOpts := []gitea.ClientOption{
		gitea.SetContext(ctx),
	}

	if opts.Token != "" {
		clientOpts = append(clientOpts, gitea.SetToken(opts.Token))
	}

	if opts.TLSConfig != nil {
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: opts.TLSConfig,
			},
		}
		clientOpts = append(clientOpts, gitea.SetHTTPClient(httpClient))
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

func (p *GiteaProvider) ListTags(ctx context.Context, opts Options) ([]Result, error) {
	giteaOpts := gitea.ListRepoTagsOptions{
		ListOptions: gitea.ListOptions{
			PageSize: 100,
		},
	}

	repoTags := make([]*gitea.Tag, 0)
	for {
		page, resp, err := p.Client.ListRepoTags(p.Owner, p.Repo, giteaOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list tags: %v", err)
		}
		repoTags = append(repoTags, page...)

		if resp.NextPage == 0 {
			break
		}
		giteaOpts.Page = resp.NextPage
	}

	tagMap := make(map[string]*gitea.Tag, len(repoTags))
	tags := make([]string, 0, len(repoTags))
	for _, tag := range repoTags {
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
			SHA: tag.Commit.SHA,
			Tag: tag.Name,
		})

		if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
			return results, nil
		}
	}
	return results, nil
}

func (p *GiteaProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	giteaOpts := gitea.ListRepoBranchesOptions{
		ListOptions: gitea.ListOptions{
			PageSize: 100,
		},
	}

	results := make([]Result, 0)
	for {
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

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
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

func (p *GiteaProvider) ListRequests(ctx context.Context, opts Options) ([]Result, error) {
	giteaOpts := gitea.ListPullRequestsOptions{
		State: gitea.StateOpen,
		ListOptions: gitea.ListOptions{
			PageSize: 100,
		},
	}

	results := make([]Result, 0)
	for {
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

			if opts.Filters.Limit > 0 && len(results) >= opts.Filters.Limit {
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
