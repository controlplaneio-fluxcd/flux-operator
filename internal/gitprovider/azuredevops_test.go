// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"os"
	"regexp"
	"testing"

	"github.com/Masterminds/semver/v3"
	git "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

// fakeAzureDevOpsClient provides configurable Azure DevOps responses for tests.
type fakeAzureDevOpsClient struct {
	getRefs         func(context.Context, git.GetRefsArgs) (*git.GetRefsResponseValue, error)
	getAnnotatedTag func(context.Context, git.GetAnnotatedTagArgs) (*git.GitAnnotatedTag, error)
	getBranches     func(context.Context, git.GetBranchesArgs) (*[]git.GitBranchStats, error)
	getPullRequests func(context.Context, git.GetPullRequestsArgs) (*[]git.GitPullRequest, error)
}

// GetRefs calls the configured refs test function.
func (f *fakeAzureDevOpsClient) GetRefs(ctx context.Context, args git.GetRefsArgs) (*git.GetRefsResponseValue, error) {
	return f.getRefs(ctx, args)
}

// GetAnnotatedTag calls the configured annotated-tag test function.
func (f *fakeAzureDevOpsClient) GetAnnotatedTag(ctx context.Context, args git.GetAnnotatedTagArgs) (*git.GitAnnotatedTag, error) {
	if f.getAnnotatedTag == nil {
		return nil, errors.New("not an annotated tag")
	}
	return f.getAnnotatedTag(ctx, args)
}

// GetBranches calls the configured branches test function.
func (f *fakeAzureDevOpsClient) GetBranches(ctx context.Context, args git.GetBranchesArgs) (*[]git.GitBranchStats, error) {
	return f.getBranches(ctx, args)
}

// GetPullRequests calls the configured pull-request test function.
func (f *fakeAzureDevOpsClient) GetPullRequests(ctx context.Context, args git.GetPullRequestsArgs) (*[]git.GitPullRequest, error) {
	return f.getPullRequests(ctx, args)
}

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
				Filters: filtering.Filters{
					SemVer: newConstraint("> 6.0.1 < 6.1.0"),
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
				Filters: filtering.Filters{
					SemVer: newConstraint("6.0.x"),
					Limit:  1,
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
				Filters: filtering.Filters{
					SemVer: newConstraint("0.0.x"),
				},
			},
			want: []Result{},
		},
		{
			name: "rejects excessive limit",
			opts: Options{
				URL: "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: filtering.Filters{
					Limit: maxGitProviderResults + 1,
				},
			},
			wantErrMsg: "exceeds maximum",
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
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`test*`),
					Exclude: regexp.MustCompile(`^feat/.*`),
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
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`ma*`),
					Limit:   1,
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
		{
			name: "rejects excessive limit",
			opts: Options{
				URL: "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: filtering.Filters{
					Limit: maxGitProviderResults + 1,
				},
			},
			wantErrMsg: "exceeds maximum",
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
				Filters: filtering.Filters{
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
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^feat/.*`),
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
		{
			name: "rejects excessive limit",
			opts: Options{
				URL: "https://dev.azure.com/stefanprodan/fluxcd-testing/_git/podinfo",
				Filters: filtering.Filters{
					Limit: maxGitProviderResults + 1,
				},
			},
			wantErrMsg: "exceeds maximum",
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

func TestAzureDevOpsProvider_TagPagination(t *testing.T) {
	t.Run("follows continuation tokens", func(t *testing.T) {
		g := NewWithT(t)
		var tokens []string
		client := &fakeAzureDevOpsClient{
			getRefs: func(_ context.Context, args git.GetRefsArgs) (*git.GetRefsResponseValue, error) {
				g.Expect(args.Top).NotTo(BeNil())
				g.Expect(*args.Top).To(Equal(gitProviderPageSize))
				token := ""
				if args.ContinuationToken != nil {
					token = *args.ContinuationToken
				}
				tokens = append(tokens, token)
				if token == "" {
					return &git.GetRefsResponseValue{
						Value: []git.GitRef{{
							Name:     new("refs/tags/v1.0.0"),
							ObjectId: new("sha-1"),
						}},
						ContinuationToken: "next",
					}, nil
				}
				return &git.GetRefsResponseValue{
					Value: []git.GitRef{{
						Name:     new("refs/tags/v2.0.0"),
						ObjectId: new("sha-2"),
					}},
				}, nil
			},
		}
		provider := &AzureDevOpsProvider{Client: client, Project: "project", Repo: "repo"}

		results, err := provider.ListTags(t.Context(), Options{Filters: filtering.Filters{Limit: 2}})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(tokens).To(Equal([]string{"", "next"}))
		g.Expect(results).To(Equal([]Result{
			{ID: inputs.ID("v2.0.0"), SHA: "sha-2", Tag: "v2.0.0"},
			{ID: inputs.ID("v1.0.0"), SHA: "sha-1", Tag: "v1.0.0"},
		}))
	})

	t.Run("rejects repeated continuation tokens", func(t *testing.T) {
		g := NewWithT(t)
		var calls int
		client := &fakeAzureDevOpsClient{
			getRefs: func(_ context.Context, _ git.GetRefsArgs) (*git.GetRefsResponseValue, error) {
				calls++
				return &git.GetRefsResponseValue{ContinuationToken: "same"}, nil
			},
		}
		provider := &AzureDevOpsProvider{Client: client, Project: "project", Repo: "repo"}

		_, err := provider.ListTags(t.Context(), Options{})
		g.Expect(err).To(MatchError(ContainSubstring("repeated continuation token")))
		g.Expect(calls).To(Equal(2))
	})
}

func TestAzureDevOpsProvider_PullRequestPagination(t *testing.T) {
	g := NewWithT(t)
	var skips []int
	client := &fakeAzureDevOpsClient{
		getPullRequests: func(_ context.Context, args git.GetPullRequestsArgs) (*[]git.GitPullRequest, error) {
			g.Expect(args.Top).NotTo(BeNil())
			g.Expect(*args.Top).To(Equal(gitProviderPageSize))
			g.Expect(args.Skip).NotTo(BeNil())
			skips = append(skips, *args.Skip)
			if *args.Skip == 0 {
				page := make([]git.GitPullRequest, gitProviderPageSize)
				return &page, nil
			}
			page := []git.GitPullRequest{{
				PullRequestId:         new(1),
				SourceRefName:         new("refs/heads/feature"),
				LastMergeSourceCommit: &git.GitCommitRef{CommitId: new("sha")},
				Title:                 new("Feature"),
				CreatedBy:             &webapi.IdentityRef{DisplayName: new("User")},
			}}
			return &page, nil
		},
	}
	provider := &AzureDevOpsProvider{Client: client, Project: "project", Repo: "repo"}

	results, err := provider.ListRequests(t.Context(), Options{Filters: filtering.Filters{Limit: 1}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(skips).To(Equal([]int{0, gitProviderPageSize}))
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].ID).To(Equal("1"))
}
