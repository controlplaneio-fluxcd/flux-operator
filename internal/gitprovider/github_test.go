// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"os"
	"regexp"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGitHubProvider_ListBranches(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "filters branches by regex",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					IncludeBranchRx: regexp.MustCompile(`^stefanprodan-patch-.*`),
					ExcludeBranchRx: regexp.MustCompile(`^stefanprodan-patch-4`),
				},
			},
			want: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Author: "",
					Title:  "",
					Branch: "stefanprodan-patch-1",
				},
				{
					ID:     "1433536418",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Author: "",
					Title:  "",
					Branch: "stefanprodan-patch-2",
				},
				{
					ID:     "1433601955",
					SHA:    "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
					Author: "",
					Title:  "",
					Branch: "stefanprodan-patch-3",
				},
			},
		},
		{
			name: "filters branches by limit",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					IncludeBranchRx: regexp.MustCompile(`^stefanprodan-patch-.*`),
					Limit:           1,
				},
			},
			want: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Author: "",
					Title:  "",
					Branch: "stefanprodan-patch-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitHubProvider(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())

			got, err := provider.ListBranches(context.Background(), tt.opts)
			if len(tt.wantErrMsg) > 0 {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErrMsg))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}

func TestGitHubProvider_ListRequests(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "all prs",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "f43c54d06a19335cb8be4607ef9a05a3b20fb485",
					Title:  "test5: Update README.md",
					Author: "stefanprodan",
					Branch: "feat/5",
				},
				{
					ID:     "4",
					SHA:    "80332195632fe293564ff563344032cf4c75af45",
					Title:  "test4: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-4",
				},
				{
					ID:     "3",
					SHA:    "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
					Title:  "test3: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-3",
				},
				{
					ID:     "2",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:  "test2: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-2",
				},
				{
					ID:     "1",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Title:  "test1: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-1",
				},
			},
		},
		{
			name: "filters prs by labels and limit",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					Limit:  2,
					Labels: []string{"enhancement"},
				},
			},
			want: []Result{
				{
					ID:     "4",
					SHA:    "80332195632fe293564ff563344032cf4c75af45",
					Title:  "test4: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-4",
				},
				{
					ID:     "2",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:  "test2: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-2",
				},
			},
		},
		{
			name: "filters prs by branches",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					IncludeBranchRx: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "f43c54d06a19335cb8be4607ef9a05a3b20fb485",
					Title:  "test5: Update README.md",
					Author: "stefanprodan",
					Branch: "feat/5",
				},
			},
		},
		{
			name: "repo not found",
			opts: Options{
				URL: "https://github.com/fluxcd-testing/invalid",
			},
			wantErrMsg: "404 Not Found",
		},
		{
			name: "wrong token",
			opts: Options{
				Token: "wrong-token",
				URL:   "https://github.com/fluxcd-testing/pr-testing",
			},
			wantErrMsg: "401 Bad credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitHubProvider(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())

			got, err := provider.ListRequests(context.Background(), tt.opts)
			if len(tt.wantErrMsg) > 0 {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErrMsg))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}
