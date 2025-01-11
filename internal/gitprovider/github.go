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

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
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
		if opts.CertPool != nil {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: opts.CertPool,
				},
			}
			ctxCA := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: tr})
			httpClient = oauth2.NewClient(ctxCA, ts)
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

func (p *GitHubProvider) ListBranches(ctx context.Context, opts Options) ([]Result, error) {
	ghOpts := &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var results []Result
	for {
		prs, resp, err := p.Client.Repositories.ListBranches(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %v", err)
		}

		for _, pr := range prs {
			if !matchBranches(opts, *pr.Name, *pr.Name) {
				continue
			}

			results = append(results, Result{
				ID:           checksum(pr.GetName()),
				SHA:          pr.GetCommit().GetSHA(),
				SourceBranch: pr.GetName(),
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

	var results []Result
	for {
		prs, resp, err := p.Client.PullRequests.List(ctx, p.Owner, p.Repo, ghOpts)
		if err != nil {
			return nil, fmt.Errorf("could not list pull requests: %v", err)
		}

		for _, pr := range prs {
			if !matchBranches(opts, pr.GetHead().GetRef(), pr.GetBase().GetRef()) {
				continue
			}

			prLabels := make([]string, len(pr.Labels))
			for i, l := range pr.Labels {
				prLabels[i] = l.GetName()
			}

			if !includesLabels(opts, prLabels) {
				continue
			}

			results = append(results, Result{
				ID:           fmt.Sprintf("%d", pr.GetNumber()),
				SHA:          pr.GetHead().GetSHA(),
				SourceBranch: pr.GetHead().GetRef(),
				TargetBranch: pr.GetBase().GetRef(),
				Title:        pr.GetTitle(),
				Author:       pr.GetUser().GetLogin(),
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
