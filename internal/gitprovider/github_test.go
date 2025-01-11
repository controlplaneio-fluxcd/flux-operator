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
					SourceBranchRx: regexp.MustCompile(`^stefanprodan-patch-.*`),
				},
			},
			want: []Result{
				{
					ID:           "1433470881",
					SHA:          "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Author:       "",
					Title:        "",
					SourceBranch: "stefanprodan-patch-1",
					TargetBranch: "",
				},
				{
					ID:           "1433536418",
					SHA:          "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Author:       "",
					Title:        "",
					SourceBranch: "stefanprodan-patch-2",
					TargetBranch: "",
				},
				{
					ID:           "1433601955",
					SHA:          "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
					Author:       "",
					Title:        "",
					SourceBranch: "stefanprodan-patch-3",
					TargetBranch: "",
				},
				{
					ID:           "1433667492",
					SHA:          "80332195632fe293564ff563344032cf4c75af45",
					Author:       "",
					Title:        "",
					SourceBranch: "stefanprodan-patch-4",
					TargetBranch: "",
				},
			},
		},
		{
			name: "filters branches by limit",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					SourceBranchRx: regexp.MustCompile(`^stefanprodan-patch-.*`),
					Limit:          1,
				},
			},
			want: []Result{
				{
					ID:           "1433470881",
					SHA:          "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Author:       "",
					Title:        "",
					SourceBranch: "stefanprodan-patch-1",
					TargetBranch: "",
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
					ID:           "5",
					SHA:          "f43c54d06a19335cb8be4607ef9a05a3b20fb485",
					Title:        "test5: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "feat/5",
					TargetBranch: "main",
				},
				{
					ID:           "4",
					SHA:          "80332195632fe293564ff563344032cf4c75af45",
					Title:        "test4: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-4",
					TargetBranch: "main",
				},
				{
					ID:           "3",
					SHA:          "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
					Title:        "test3: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-3",
					TargetBranch: "main",
				},
				{
					ID:           "2",
					SHA:          "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:        "test2: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-2",
					TargetBranch: "main",
				},
				{
					ID:           "1",
					SHA:          "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Title:        "test1: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-1",
					TargetBranch: "main",
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
					ID:           "4",
					SHA:          "80332195632fe293564ff563344032cf4c75af45",
					Title:        "test4: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-4",
					TargetBranch: "main",
				},
				{
					ID:           "2",
					SHA:          "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:        "test2: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "stefanprodan-patch-2",
					TargetBranch: "main",
				},
			},
		},
		{
			name: "filters prs by branches",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					SourceBranchRx: regexp.MustCompile(`^feat/.*`),
					TargetBranchRx: regexp.MustCompile(`main`),
				},
			},
			want: []Result{
				{
					ID:           "5",
					SHA:          "f43c54d06a19335cb8be4607ef9a05a3b20fb485",
					Title:        "test5: Update README.md",
					Author:       "stefanprodan",
					SourceBranch: "feat/5",
					TargetBranch: "main",
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
