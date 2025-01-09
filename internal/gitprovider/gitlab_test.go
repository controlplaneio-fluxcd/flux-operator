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

func TestGitLabProvider_ListRequests(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "all mrs",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
			},
			want: []Result{
				{
					ID:           "5",
					SHA:          "3fd0d45b23e5f14089587a9049e33d82497b944b",
					Author:       "stefanprodan",
					Title:        "test5: Edit README.md",
					SourceBranch: "feat/5",
					TargetBranch: "master",
				},
				{
					ID:           "4",
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					Author:       "stefanprodan",
					Title:        "test4: Edit README.md",
					SourceBranch: "patch-4",
					TargetBranch: "master",
				},
				{
					ID:           "3",
					SHA:          "f2aed00334494f13d92d065ecda39aea0d0b871f",
					Author:       "stefanprodan",
					Title:        "test3: Edit README.md",
					SourceBranch: "patch-3",
					TargetBranch: "master",
				},
				{
					ID:           "2",
					SHA:          "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					Author:       "stefanprodan",
					Title:        "test2: Edit README.md",
					SourceBranch: "patch-2",
					TargetBranch: "master",
				},
				{
					ID:           "1",
					SHA:          "cebef2d870bc83b37f43c470bae205fca094bacc",
					Author:       "stefanprodan",
					Title:        "test1: Edit README.md",
					SourceBranch: "patch-1",
					TargetBranch: "master",
				},
			},
		},
		{
			name: "filters mrs by labels and limit",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					Limit:  2,
					Labels: []string{"enhancement"},
				},
			},
			want: []Result{
				{
					ID:           "4",
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					Author:       "stefanprodan",
					Title:        "test4: Edit README.md",
					SourceBranch: "patch-4",
					TargetBranch: "master",
				},
				{
					ID:           "2",
					SHA:          "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					Author:       "stefanprodan",
					Title:        "test2: Edit README.md",
					SourceBranch: "patch-2",
					TargetBranch: "master",
				},
			},
		},
		{
			name: "filters mrs by branches",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					SourceBranchRx: regexp.MustCompile(`^feat/.*`),
					TargetBranchRx: regexp.MustCompile(`master`),
				},
			},
			want: []Result{
				{
					ID:           "5",
					SHA:          "3fd0d45b23e5f14089587a9049e33d82497b944b",
					Author:       "stefanprodan",
					Title:        "test5: Edit README.md",
					SourceBranch: "feat/5",
					TargetBranch: "master",
				},
			},
		},
		{
			name: "repo not found",
			opts: Options{
				URL: "https://gitlab.com/stefanprodan/invalid",
			},
			wantErrMsg: "404 Not Found",
		},
		{
			name: "wrong token",
			opts: Options{
				Token: "wrong-token",
				URL:   "https://gitlab.com/stefanprodan/podinfo",
			},
			wantErrMsg: "401 Unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitLabProvider(context.Background(), tt.opts)
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
