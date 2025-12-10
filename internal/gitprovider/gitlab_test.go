// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"errors"
	"regexp"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
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
		tags       []*gitlab.Tag
		listError  error
		want       []Result
		wantErrMsg string
	}{
		{
			name: "filters tags by semver",
			opts: Options{
				Filters: filtering.Filters{
					SemVer: newConstraint("5.0.x"),
				},
			},
			tags: []*gitlab.Tag{
				{Name: "6.0.4", Commit: &gitlab.Commit{ID: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
				{Name: "5.1.0", Commit: &gitlab.Commit{ID: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
				{Name: "5.0.3", Commit: &gitlab.Commit{ID: "95be17be1dc2103eb5e2c0b0bac50ef692c4657d"}},
				{Name: "5.0.2", Commit: &gitlab.Commit{ID: "6596ed08de58bffc6982512a0483be3b2ec346ce"}},
				{Name: "5.0.1", Commit: &gitlab.Commit{ID: "7411da595c25183daba255068814b83843fe3395"}},
				{Name: "5.0.0", Commit: &gitlab.Commit{ID: "9299a2d1f300267354609bee398caa2cb5548594"}},
				{Name: "4.1.0", Commit: &gitlab.Commit{ID: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
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
				Filters: filtering.Filters{
					SemVer: newConstraint("6.0.x"),
					Limit:  1,
				},
			},
			tags: []*gitlab.Tag{
				{Name: "7.0.0", Commit: &gitlab.Commit{ID: "7411da595c25183daba255068814b83843fe3395"}},
				{Name: "6.1.0", Commit: &gitlab.Commit{ID: "7411da595c25183daba255068814b83843fe3395"}},
				{Name: "6.0.4", Commit: &gitlab.Commit{ID: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
				{Name: "6.0.3", Commit: &gitlab.Commit{ID: "95be17be1dc2103eb5e2c0b0bac50ef692c4657d"}},
				{Name: "6.0.2", Commit: &gitlab.Commit{ID: "6596ed08de58bffc6982512a0483be3b2ec346ce"}},
				{Name: "6.0.1", Commit: &gitlab.Commit{ID: "7411da595c25183daba255068814b83843fe3395"}},
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
				Filters: filtering.Filters{
					Limit: 1,
				},
			},
			tags: []*gitlab.Tag{
				{Name: "v1.8.0", Commit: &gitlab.Commit{ID: "6c8a85a5ab953874c7c83d50317359a0e5a352a9"}},
				{Name: "6.1.0", Commit: &gitlab.Commit{ID: "7411da595c25183daba255068814b83843fe3395"}},
				{Name: "6.0.4", Commit: &gitlab.Commit{ID: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
			},
			want: []Result{
				{
					ID:  "95093100",
					SHA: "6c8a85a5ab953874c7c83d50317359a0e5a352a9",
					Tag: "v1.8.0",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			project := "example-org/example"
			mockClient := gitlabtesting.NewTestClient(t)

			mockClient.MockTags.EXPECT().
				ListTags(project, &gitlab.ListTagsOptions{
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
					},
				}).
				Return(tt.tags, &gitlab.Response{}, tt.listError)

			provider := GitLabProvider{Client: mockClient.Client, Project: project}

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
		name          string
		opts          Options
		branches      []*gitlab.Branch
		expectedRegex *string
		listError     error
		want          []Result
		wantErrMsg    string
	}{
		{
			name: "filters branches by regex",
			opts: Options{
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^patch-.*`),
					Exclude: regexp.MustCompile(`^patch-4`),
				},
			},
			branches: []*gitlab.Branch{
				{Name: "patch-1", Commit: &gitlab.Commit{ID: "cebef2d870bc83b37f43c470bae205fca094bacc"}},
				{Name: "patch-2", Commit: &gitlab.Commit{ID: "a275fb0322466eaa1a74485a4f79f88d7c8858e8"}},
				{Name: "random-branch", Commit: &gitlab.Commit{ID: "dba7673010f19a94af4345453005933fd511bea9"}},
				{Name: "patch-3", Commit: &gitlab.Commit{ID: "f2aed00334494f13d92d065ecda39aea0d0b871f"}},
				{Name: "patch-4", Commit: &gitlab.Commit{ID: "4e1243bd22c66e76c2ba9eddc1f91394e57f9f83"}},
			},
			expectedRegex: gitlab.Ptr("^patch-.*"),
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
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^patch-.*`),
					Limit:   1,
				},
			},
			branches: []*gitlab.Branch{
				{Name: "patch-1", Commit: &gitlab.Commit{ID: "cebef2d870bc83b37f43c470bae205fca094bacc"}},
				{Name: "patch-2", Commit: &gitlab.Commit{ID: "a275fb0322466eaa1a74485a4f79f88d7c8858e8"}},
				{Name: "random-branch", Commit: &gitlab.Commit{ID: "dba7673010f19a94af4345453005933fd511bea9"}},
				{Name: "patch-3", Commit: &gitlab.Commit{ID: "f2aed00334494f13d92d065ecda39aea0d0b871f"}},
				{Name: "patch-4", Commit: &gitlab.Commit{ID: "4e1243bd22c66e76c2ba9eddc1f91394e57f9f83"}},
			},
			expectedRegex: gitlab.Ptr("^patch-.*"),
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

			project := "example-org/example"
			mockClient := gitlabtesting.NewTestClient(t)

			mockClient.MockBranches.EXPECT().
				ListBranches(project, &gitlab.ListBranchesOptions{
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
					},
					Regex: tt.expectedRegex,
				}).
				Return(tt.branches, &gitlab.Response{}, tt.listError)

			provider := GitLabProvider{Client: mockClient.Client, Project: project}

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
		name                 string
		opts                 Options
		mergeRequests        []*gitlab.BasicMergeRequest
		expectedLabelOptions *gitlab.LabelOptions
		listError            error
		want                 []Result
		wantErrMsg           string
	}{
		{
			name: "all mrs",
			mergeRequests: []*gitlab.BasicMergeRequest{
				{
					IID:          5,
					SHA:          "3fd0d45b23e5f14089587a9049e33d82497b944b",
					SourceBranch: "feat/5",
					Title:        "test5: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{},
				},
				{
					IID:          4,
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					SourceBranch: "patch-4",
					Title:        "test4: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"documentation", "enhancement"},
				},
				{
					IID:          3,
					SHA:          "f2aed00334494f13d92d065ecda39aea0d0b871f",
					SourceBranch: "patch-3",
					Title:        "test3: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"documentation"},
				},
				{
					IID:          2,
					SHA:          "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					SourceBranch: "patch-2",
					Title:        "test2: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
				},
				{
					IID:          1,
					SHA:          "cebef2d870bc83b37f43c470bae205fca094bacc",
					SourceBranch: "patch-1",
					Title:        "test1: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
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
				Filters: filtering.Filters{
					Limit:  2,
					Labels: []string{"enhancement"},
				},
			},
			expectedLabelOptions: gitlab.Ptr(gitlab.LabelOptions([]string{"enhancement"})),
			mergeRequests: []*gitlab.BasicMergeRequest{
				{
					IID:          4,
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					SourceBranch: "patch-4",
					Title:        "test4: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"documentation", "enhancement"},
				},
				{
					IID:          2,
					SHA:          "a275fb0322466eaa1a74485a4f79f88d7c8858e8",
					SourceBranch: "patch-2",
					Title:        "test2: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
				},
				{
					IID:          1,
					SHA:          "cebef2d870bc83b37f43c470bae205fca094bacc",
					SourceBranch: "patch-1",
					Title:        "test1: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
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
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^feat/.*`),
				},
			},
			mergeRequests: []*gitlab.BasicMergeRequest{
				{
					IID:          5,
					SHA:          "3fd0d45b23e5f14089587a9049e33d82497b944b",
					SourceBranch: "feat/5",
					Title:        "test5: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{},
				},
				{
					IID:          4,
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					SourceBranch: "patch-4",
					Title:        "test4: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"documentation", "enhancement"},
				},
				{
					IID:          3,
					SHA:          "f2aed00334494f13d92d065ecda39aea0d0b871f",
					SourceBranch: "patch-3",
					Title:        "test3: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"documentation"},
				},
				{
					IID:          2,
					SHA:          "a143f78b7f8abd511a4f4ce84b4875edfb621a56",
					SourceBranch: "patch-2",
					Title:        "test2: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
				},
				{
					IID:          1,
					SHA:          "cebef2d870bc83b37f43c470bae205fca094bacc",
					SourceBranch: "patch-1",
					Title:        "test1: Edit README.md",
					Author:       &gitlab.BasicUser{Username: "stefanprodan"},
					Labels:       []string{"enhancement"},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			project := "example-org/example"
			mockClient := gitlabtesting.NewTestClient(t)

			mockClient.MockMergeRequests.EXPECT().
				ListProjectMergeRequests(project, &gitlab.ListProjectMergeRequestsOptions{
					State:  gitlab.Ptr("opened"),
					Labels: tt.expectedLabelOptions,
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
					},
				}).
				Return(tt.mergeRequests, &gitlab.Response{}, tt.listError)

			provider := GitLabProvider{Client: mockClient.Client, Project: project}

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

type gitlabEnvironmentResponse struct {
	gitlab.Environment
	deploymentListError error
	deployments         []*gitlab.Deployment
}

func TestGitLabProvider_ListEnvironments(t *testing.T) {
	tests := []struct {
		name         string
		opts         Options
		environments []*gitlabEnvironmentResponse
		listError    error
		want         []Result
		wantErrMsg   string
	}{
		{
			name: "all environments",
			opts: Options{},
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 2, Name: "env2", Slug: "env2-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 3, Name: "env3", Slug: "env3-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
			},
			want: []Result{
				{
					ID:     "1",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "env1",
					Slug:   "env1-slug",
					Branch: "branch1",
				},
				{
					ID:     "2",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "env2",
					Slug:   "env2-slug",
					Branch: "branch1",
				},
				{
					ID:     "3",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "env3",
					Slug:   "env3-slug",
					Branch: "branch1",
				},
			},
		},
		{
			name: "filters environments by name and limit",
			opts: Options{
				Filters: filtering.Filters{
					Limit:   2,
					Include: regexp.MustCompile(`^review/.*`),
					Exclude: regexp.MustCompile(`^review/do-not-deploy.*`),
				},
			},
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "review/env1", Slug: "env1-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 2, Name: "env2", Slug: "env2-slug", State: "available"},
				},
				{
					Environment: gitlab.Environment{ID: 3, Name: "review/do-not-deploy-env3", Slug: "env3-slug", State: "available"},
				},
				{
					Environment: gitlab.Environment{ID: 4, Name: "review/env4", Slug: "env4-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 5, Name: "review/env5", Slug: "env5-slug", State: "available"},
				},
			},
			want: []Result{
				{
					ID:     "1",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "review/env1",
					Slug:   "env1-slug",
					Branch: "branch1",
				},
				{
					ID:     "4",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "review/env4",
					Slug:   "env4-slug",
					Branch: "branch1",
				},
			},
		},
		{
			name: "uses data from latest successful or running deployment",
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author1"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "failed",
						},
						{
							User: &gitlab.ProjectUser{Username: "example-author2"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha2"},
								Ref:    "branch1",
							},
							Status: "success",
						},
						{
							User: &gitlab.ProjectUser{Username: "example-author3"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha3"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 2, Name: "env2", Slug: "env2-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author1"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "failed",
						},
						{
							User: &gitlab.ProjectUser{Username: "example-author2"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha2"},
								Ref:    "branch1",
							},
							Status: "running",
						},
						{
							User: &gitlab.ProjectUser{Username: "example-author3"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha3"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
			},
			want: []Result{
				{
					ID:     "1",
					SHA:    "sha2",
					Author: "example-author2",
					Title:  "env1",
					Slug:   "env1-slug",
					Branch: "branch1",
				},
				{
					ID:     "2",
					SHA:    "sha2",
					Author: "example-author2",
					Title:  "env2",
					Slug:   "env2-slug",
					Branch: "branch1",
				},
			},
		},
		{
			name: "excludes stopped environment without running deployments",
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "stopped"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "success",
						},
					},
				},
			},
			want: []Result{},
		},
		{
			name: "excludes environments without successful or any deployments",
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "available"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "failed",
						},
					},
				},
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "available"},
					deployments: []*gitlab.Deployment{},
				},
			},
			want: []Result{},
		},
		{
			name: "uses data from latest running deployment even for stopped environment",
			environments: []*gitlabEnvironmentResponse{
				{
					Environment: gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "stopped"},
					deployments: []*gitlab.Deployment{
						{
							User: &gitlab.ProjectUser{Username: "example-author"},
							Deployable: gitlab.DeploymentDeployable{
								Commit: &gitlab.Commit{ID: "sha1"},
								Ref:    "branch1",
							},
							Status: "running",
						},
					},
				},
			},
			want: []Result{
				{
					ID:     "1",
					SHA:    "sha1",
					Author: "example-author",
					Title:  "env1",
					Slug:   "env1-slug",
					Branch: "branch1",
				},
			},
		},
		{
			name:       "propagates environment list error",
			listError:  errors.New("some error"),
			wantErrMsg: "could not list environments: some error",
		},
		{
			name: "propagates deployment list error",
			environments: []*gitlabEnvironmentResponse{
				{
					Environment:         gitlab.Environment{ID: 1, Name: "env1", Slug: "env1-slug", State: "available"},
					deploymentListError: errors.New("some error"),
				},
			},
			wantErrMsg: `could not list deployments for environment "env1": some error`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			project := "example-org/example"
			mockClient := gitlabtesting.NewTestClient(t)

			var envResponses []*gitlab.Environment
			for _, env := range tt.environments {
				envResponses = append(envResponses, &env.Environment)

				// Explicitly check for nil to skip adding mock - use non-nil empty slice for empty response
				if env.deployments == nil && env.deploymentListError == nil {
					continue
				}

				mockClient.MockDeployments.EXPECT().
					ListProjectDeployments(project, &gitlab.ListProjectDeploymentsOptions{
						ListOptions: gitlab.ListOptions{},
						OrderBy:     gitlab.Ptr("created_at"),
						Sort:        gitlab.Ptr("desc"),
						Environment: gitlab.Ptr(env.Name),
					}).
					Return(env.deployments, &gitlab.Response{}, env.deploymentListError)
			}
			mockClient.MockEnvironments.EXPECT().
				ListEnvironments(project, &gitlab.ListEnvironmentsOptions{
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
					},
				}).
				Return(envResponses, &gitlab.Response{}, tt.listError)

			provider := GitLabProvider{Client: mockClient.Client, Project: project}

			got, err := provider.ListEnvironments(context.Background(), tt.opts)
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
