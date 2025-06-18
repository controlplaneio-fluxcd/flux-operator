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

func TestGitHubProvider_ListTags(t *testing.T) {
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
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					SemverConstraints: newConstraint("> 6.0.1 < 6.1.0"),
				},
			},
			want: []Result{
				{
					ID:  "48955639",
					SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009",
					Tag: "6.0.4",
				},
				{
					ID:  "48890102",
					SHA: "ea292aa958c5e348266518af2261dc04d6270439",
					Tag: "6.0.3",
				},
				{
					ID:  "48824565",
					SHA: "693ffa9d28208c677738a0e2061f41694dfaa183",
					Tag: "6.0.2",
				},
			},
		},
		{
			name: "filters tags by semver and limit",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
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
			name: "sorts tags alphabetically in ascending order",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					Alphabetical: "asc",
					Limit:        3,
				},
			},
			want: []Result{
				{
					ID:  "47251697",
					SHA: "81ef8654bff2d5fba88dcf9cf874b3adc425d6b7",
					Tag: "0.2.2",
				},
				{
					ID:  "47382767",
					SHA: "dc27269a47e66923dbd8bdecf465a23092097f9e",
					Tag: "2.0.0",
				},
				{
					ID:  "47448304",
					SHA: "e0864b6e205dc7755cd7b2f47f89273b31f7189d",
					Tag: "2.0.1",
				},
			},
		},
		{
			name: "sorts tags alphabetically in descending order",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					Alphabetical: "desc",
					Limit:        3,
				},
			},
			want: []Result{
				{
					ID:  "95093100",
					SHA: "6c8a85a5ab953874c7c83d50317359a0e5a352a9",
					Tag: "v1.8.0",
				},
				{
					ID:  "94896491",
					SHA: "18af1ea3a6c340c252e97b7875e929a97e7b0b8f",
					Tag: "v1.7.0",
				},
				{
					ID:  "94699882",
					SHA: "44f588dd4c76ee5d78f7865ab016510c901096da",
					Tag: "v1.6.0",
				},
			},
		},
		{
			name: "sorts tags numerically in ascending order",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					Numerical: "asc",
					Limit:     3,
				},
			},
			want: []Result{
				{
					ID:  "95093100",
					SHA: "6c8a85a5ab953874c7c83d50317359a0e5a352a9",
					Tag: "123456",
				},
				{
					ID:  "94896491",
					SHA: "18af1ea3a6c340c252e97b7875e929a97e7b0b8f",
					Tag: "234567",
				},
				{
					ID:  "94699882",
					SHA: "44f588dd4c76ee5d78f7865ab016510c901096da",
					Tag: "345678",
				},
			},
		},
		{
			name: "sorts tags numerically in descending order",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					Numerical: "desc",
					Limit:     3,
				},
			},
			want: []Result{
				{
					ID:  "94699882",
					SHA: "44f588dd4c76ee5d78f7865ab016510c901096da",
					Tag: "345678",
				},
				{
					ID:  "94896491",
					SHA: "18af1ea3a6c340c252e97b7875e929a97e7b0b8f",
					Tag: "234567",
				},
				{
					ID:  "95093100",
					SHA: "6c8a85a5ab953874c7c83d50317359a0e5a352a9",
					Tag: "123456",
				},
			},
		},
		{
			name: "filters tags no results",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/stefanprodan/podinfo",
				Filters: Filters{
					SemverConstraints: newConstraint("0.0.x"),
				},
			},
			want: []Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewGitHubProvider(context.Background(), tt.opts)
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
					IncludeBranchRe: regexp.MustCompile(`^stefanprodan-patch-.*`),
					ExcludeBranchRe: regexp.MustCompile(`^stefanprodan-patch-4`),
				},
			},
			want: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Branch: "stefanprodan-patch-1",
				},
				{
					ID:     "1433536418",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Branch: "stefanprodan-patch-2",
				},
				{
					ID:     "1433601955",
					SHA:    "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
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
					IncludeBranchRe: regexp.MustCompile(`^stefanprodan-patch-.*`),
					Limit:           1,
				},
			},
			want: []Result{
				{
					ID:     "1433470881",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
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
					Labels: []string{},
				},
				{
					ID:     "4",
					SHA:    "80332195632fe293564ff563344032cf4c75af45",
					Title:  "test4: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-4",
					Labels: []string{"documentation", "enhancement"},
				},
				{
					ID:     "3",
					SHA:    "29d1d3a726e1e1f68b7cb60ac891cb83fa260ea9",
					Title:  "test3: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-3",
					Labels: []string{"documentation"},
				},
				{
					ID:     "2",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:  "test2: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-2",
					Labels: []string{"enhancement"},
				},
				{
					ID:     "1",
					SHA:    "2dd3a8d2088457e5cf991018edf13e25cbd61380",
					Title:  "test1: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-1",
					Labels: []string{"enhancement"},
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
					Labels: []string{"documentation", "enhancement"},
				},
				{
					ID:     "2",
					SHA:    "1e5aef14d38a8c67e5240308adf2935d6cdc2ec8",
					Title:  "test2: Update README.md",
					Author: "stefanprodan",
					Branch: "stefanprodan-patch-2",
					Labels: []string{"enhancement"},
				},
			},
		},
		{
			name: "filters prs by branches",
			opts: Options{
				Token: os.Getenv("GITHUB_TOKEN"),
				URL:   "https://github.com/fluxcd-testing/pr-testing",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "f43c54d06a19335cb8be4607ef9a05a3b20fb485",
					Title:  "test5: Update README.md",
					Author: "stefanprodan",
					Branch: "feat/5",
					Labels: []string{},
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
