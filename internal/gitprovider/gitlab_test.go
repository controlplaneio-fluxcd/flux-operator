// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/gomega"
)

func TestGitLabProvider_ListTags(t *testing.T) {
	newConstraint := func(s string) *semver.Constraints {
		c, err := semver.NewConstraint(s)
		if err != nil {
			panic(err)
		}
		return c
	}
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "filters tags by semver",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					SemverConstraints: newConstraint("5.0.x"),
				},
			},
			want: []Result{
				{
					ID:  "48562421",
					SHA: "95be17be1dc2103eb5e2c0b0bac50ef692c4657d",
					Tag: "5.0.3",
				},
				{
					ID:  "48496884",
					SHA: "6596ed08de58bffc6982512a0483be3b2ec346ce",
					Tag: "5.0.2",
				},
				{
					ID:  "48431347",
					SHA: "7411da595c25183daba255068814b83843fe3395",
					Tag: "5.0.1",
				},
				{
					ID:  "48365810",
					SHA: "9299a2d1f300267354609bee398caa2cb5548594",
					Tag: "5.0.0",
				},
			},
		},
		{
			name: "filters tags by semver and limit",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					SemverConstraints: newConstraint("6.0.x"),
					Limit:             1,
				},
			},
			want: []Result{
				{
					ID:  "48955639",
					SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009",
					Tag: "6.0.4",
				},
			},
		},
		{
			name: "filters tags by limit",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					Limit: 1,
				},
			},
			want: []Result{
				{
					ID:  "49283322",
					SHA: "450796ddb2ab6724ee1cc32a4be56da032d1cca0",
					Tag: "6.1.6",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitLabProvider(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())

			got, err := provider.ListTags(context.Background(), tt.opts)
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

func TestGitLabProvider_ListBranches(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "filters branches by regex",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`^patch-.*`),
					ExcludeBranchRe: regexp.MustCompile(`^patch-4`),
				},
			},
			want: []Result{
				{
					ID:     "183501423",
					SHA:    "cebef2d870bc83b37f43c470bae205fca094bacc",
					Author: "",
					Title:  "",
					Branch: "patch-1",
				},
				{
					ID:     "183566960",
					SHA:    "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					Author: "",
					Title:  "",
					Branch: "patch-2",
				},
				{
					ID:     "183632497",
					SHA:    "f2aed00334494f13d92d065ecda39aea0d0b871f",
					Author: "",
					Title:  "",
					Branch: "patch-3",
				},
			},
		},
		{
			name: "filters branches by limit",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`^patch-.*`),
					Limit:           1,
				},
			},
			want: []Result{
				{
					ID:     "183501423",
					SHA:    "cebef2d870bc83b37f43c470bae205fca094bacc",
					Author: "",
					Title:  "",
					Branch: "patch-1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitLabProvider(context.Background(), tt.opts)
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
					ID:     "5",
					SHA:    "3fd0d45b23e5f14089587a9049e33d82497b944b",
					Author: "stefanprodan",
					Title:  "test5: Edit README.md",
					Branch: "feat/5",
					Labels: []string{},
				},
				{
					ID:     "4",
					SHA:    "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					Author: "stefanprodan",
					Title:  "test4: Edit README.md",
					Branch: "patch-4",
					Labels: []string{"documentation", "enhancement"},
				},
				{
					ID:     "3",
					SHA:    "f2aed00334494f13d92d065ecda39aea0d0b871f",
					Author: "stefanprodan",
					Title:  "test3: Edit README.md",
					Branch: "patch-3",
					Labels: []string{"documentation"},
				},
				{
					ID:     "2",
					SHA:    "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					Author: "stefanprodan",
					Title:  "test2: Edit README.md",
					Branch: "patch-2",
					Labels: []string{"enhancement"},
				},
				{
					ID:     "1",
					SHA:    "cebef2d870bc83b37f43c470bae205fca094bacc",
					Author: "stefanprodan",
					Title:  "test1: Edit README.md",
					Branch: "patch-1",
					Labels: []string{"enhancement"},
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
					ID:     "4",
					SHA:    "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					Author: "stefanprodan",
					Title:  "test4: Edit README.md",
					Branch: "patch-4",
					Labels: []string{"documentation", "enhancement"},
				},
				{
					ID:     "2",
					SHA:    "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					Author: "stefanprodan",
					Title:  "test2: Edit README.md",
					Branch: "patch-2",
					Labels: []string{"enhancement"},
				},
			},
		},
		{
			name: "filters mrs by branches",
			opts: Options{
				Token: os.Getenv("GITLAB_TOKEN"),
				URL:   "https://gitlab.com/stefanprodan/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "3fd0d45b23e5f14089587a9049e33d82497b944b",
					Author: "stefanprodan",
					Title:  "test5: Edit README.md",
					Branch: "feat/5",
					Labels: []string{},
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
