// Copyright 2025 Stefan Prodan.
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

func TestAzureDevOpsProvider_ListTags(t *testing.T) {
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
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
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
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
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
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
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

			provider, err := NewAzureDevOpsProvider(context.Background(), tt.opts)
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

func TestAzureDevOpsProvider_ListBranches(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "filters branches by regex",
			opts: Options{
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`test*`),
					ExcludeBranchRe: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{
					ID:     "105841138",
					SHA:    "d233e53524b51c38f974f672d993bd8e6635f2cf",
					Branch: "test1",
				},
				{
					ID:     "105906675",
					SHA:    "474beb4fe680877b42d22b66b921872c3aba6be3",
					Branch: "test2",
				},
				{
					ID:     "105972212",
					SHA:    "abcad46a4437ca375e8533d42b9dc2609434aefe",
					Branch: "test3",
				},
				{
					ID:     "106037749",
					SHA:    "d217582ec33a4697773631f0d59f45e1c7a915ec",
					Branch: "test4",
				},
			},
		},
		{
			name: "filters branches by limit",
			opts: Options{
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`ma*`),
					Limit:           1,
				},
			},
			want: []Result{
				{
					ID:     "148701837",
					SHA:    "c23d57a4e998016075ad67f4ad0e6c607e012fc8",
					Branch: "master",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewAzureDevOpsProvider(context.Background(), tt.opts)
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

func TestAzureDevOpsProvider_ListRequests(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		want       []Result
		wantErrMsg string
	}{
		{
			name: "all prs",
			opts: Options{
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "2c63b750ee7520429fbf49f2867859211f3c94aa",
					Branch: "feat/5",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "New feature test",
					Labels: []string{"feat"},
				},
				{
					ID:     "4",
					SHA:    "d217582ec33a4697773631f0d59f45e1c7a915ec",
					Branch: "test4",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 4",
					Labels: []string{"fix", "typo"},
				},
				{
					ID:     "3",
					SHA:    "abcad46a4437ca375e8533d42b9dc2609434aefe",
					Branch: "test3",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 3",
					Labels: []string{"fix"},
				},
				{
					ID:     "2",
					SHA:    "474beb4fe680877b42d22b66b921872c3aba6be3",
					Branch: "test2",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 2",
					Labels: []string{"fix"},
				},
				{
					ID:     "1",
					SHA:    "d233e53524b51c38f974f672d993bd8e6635f2cf",
					Branch: "test1",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 1",
					Labels: []string{"fix", "typo"},
				},
			},
		},
		{
			name: "filters prs by labels and limit",
			opts: Options{
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: Filters{
					Limit:  2,
					Labels: []string{"fix"},
				},
			},
			want: []Result{
				{
					ID:     "4",
					SHA:    "d217582ec33a4697773631f0d59f45e1c7a915ec",
					Branch: "test4",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 4",
					Labels: []string{"fix", "typo"},
				},
				{
					ID:     "3",
					SHA:    "abcad46a4437ca375e8533d42b9dc2609434aefe",
					Branch: "test3",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "Test 3",
					Labels: []string{"fix"},
				},
			},
		},
		{
			name: "filters prs by branches",
			opts: Options{
				Token: os.Getenv("AZURE_TOKEN"),
				URL:   "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: Filters{
					IncludeBranchRe: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{
					ID:     "5",
					SHA:    "2c63b750ee7520429fbf49f2867859211f3c94aa",
					Branch: "feat/5",
					Tag:    "",
					Author: "Stefan Prodan",
					Title:  "New feature test",
					Labels: []string{"feat"},
				},
			},
		},
		{
			name: "repo not found",
			opts: Options{
				URL: "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/doesnotexist",
			},
			wantErrMsg: "could not list pull requests: TF401019: The Git repository with name or identifier doesnotexist does not exist or you do not have permissions for the operation you are attempting.",
		},
		{
			name: "invalid token using random private azure dev ops repo",
			opts: Options{
				Token: "wrong-token",
				URL:   "https://dev.azure.com/acme-corp/infrastructure-project/_git/terraform-modules",
			},
			wantErrMsg: "The user 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa' is not authorized to access this resource.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			provider, err := NewAzureDevOpsProvider(context.Background(), tt.opts)
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
