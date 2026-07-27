// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"regexp"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

// fakeAWSCodeCommitClient provides configurable CodeCommit responses for tests.
type fakeAWSCodeCommitClient struct {
	listPullRequests func(context.Context, *codecommit.ListPullRequestsInput) (*codecommit.ListPullRequestsOutput, error)
	getPullRequest   func(context.Context, *codecommit.GetPullRequestInput) (*codecommit.GetPullRequestOutput, error)
}

// ListPullRequests calls the configured pull-request list test function.
func (f *fakeAWSCodeCommitClient) ListPullRequests(ctx context.Context, input *codecommit.ListPullRequestsInput,
	_ ...func(*codecommit.Options)) (*codecommit.ListPullRequestsOutput, error) {
	return f.listPullRequests(ctx, input)
}

// GetPullRequest calls the configured pull-request detail test function.
func (f *fakeAWSCodeCommitClient) GetPullRequest(ctx context.Context, input *codecommit.GetPullRequestInput,
	_ ...func(*codecommit.Options)) (*codecommit.GetPullRequestOutput, error) {
	return f.getPullRequest(ctx, input)
}

func TestParseAWSCodeCommitURL(t *testing.T) {
	for _, tt := range []struct {
		name      string
		url       string
		wantHost  string
		wantReg   string
		wantRepo  string
		wantError bool
	}{
		{
			name:     "valid URL",
			url:      "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantHost: "https://git-codecommit.us-east-1.amazonaws.com",
			wantReg:  "us-east-1",
			wantRepo: "my-repo",
		},
		{
			name:     "valid URL with eu-west-1",
			url:      "https://git-codecommit.eu-west-1.amazonaws.com/v1/repos/flux-codecommit-repo",
			wantHost: "https://git-codecommit.eu-west-1.amazonaws.com",
			wantReg:  "eu-west-1",
			wantRepo: "flux-codecommit-repo",
		},
		{
			name:     "valid FIPS URL",
			url:      "https://git-codecommit-fips.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantHost: "https://git-codecommit-fips.us-east-1.amazonaws.com",
			wantReg:  "us-east-1",
			wantRepo: "my-repo",
		},
		{
			name:      "invalid scheme",
			url:       "http://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantError: true,
		},
		{
			name:      "SSH URL",
			url:       "ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/my-repo",
			wantError: true,
		},
		{
			name:      "invalid host",
			url:       "https://github.com/owner/repo",
			wantError: true,
		},
		{
			name:      "invalid path - missing v1/repos",
			url:       "https://git-codecommit.us-east-1.amazonaws.com/repos/my-repo",
			wantError: true,
		},
		{
			name:      "invalid path - no repo name",
			url:       "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/",
			wantError: true,
		},
		{
			name:      "empty URL",
			url:       "",
			wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			host, region, repo, err := parseAWSCodeCommitURL(tt.url)
			if tt.wantError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(host).To(Equal(tt.wantHost))
			g.Expect(region).To(Equal(tt.wantReg))
			g.Expect(repo).To(Equal(tt.wantRepo))
		})
	}
}

func TestParseAWSCodeCommitRegion(t *testing.T) {
	g := NewWithT(t)

	region, err := ParseAWSCodeCommitRegion("https://git-codecommit.ap-southeast-1.amazonaws.com/v1/repos/test")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(region).To(Equal("ap-southeast-1"))

	_, err = ParseAWSCodeCommitRegion("https://github.com/owner/repo")
	g.Expect(err).To(HaveOccurred())
}

func TestParseGoGitTags(t *testing.T) {
	g := NewWithT(t)

	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.0.0"), plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")),
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.1.0"), plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")),    // Annotated tag object hash
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.1.0^{}"), plumbing.NewHash("cccccccccccccccccccccccccccccccccccccccc")), // Annotated tag commit hash
	}

	filters := filtering.Filters{
		SemVer: newConstraint(">= 1.0.0"),
	}

	results, err := parseGoGitTags(refs, filters)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(results).To(HaveLen(2))

	// v1.1.0 should have the peeled commit hash, NOT the tag object hash.
	g.Expect(results[0].Tag).To(Equal("v1.1.0"))
	g.Expect(results[0].SHA).To(Equal("cccccccccccccccccccccccccccccccccccccccc"))

	// v1.0.0 is a lightweight tag.
	g.Expect(results[1].Tag).To(Equal("v1.0.0"))
	g.Expect(results[1].SHA).To(Equal("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
}

func TestParseGoGitBranches(t *testing.T) {
	g := NewWithT(t)

	refs := []*plumbing.Reference{
		plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")),
		plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/feature-x"), plumbing.NewHash("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")),
		plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/feature-y"), plumbing.NewHash("cccccccccccccccccccccccccccccccccccccccc")),
		plumbing.NewHashReference(plumbing.ReferenceName("refs/tags/v1.0.0"), plumbing.NewHash("dddddddddddddddddddddddddddddddddddddddd")),
		plumbing.NewSymbolicReference(plumbing.ReferenceName("HEAD"), plumbing.ReferenceName("refs/heads/main")),
	}

	// No filters: all branches returned.
	results, err := parseGoGitBranches(refs, filtering.Filters{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(results).To(HaveLen(3))
	g.Expect(results[0].Branch).To(Equal("main"))
	g.Expect(results[0].SHA).To(Equal("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	g.Expect(results[1].Branch).To(Equal("feature-x"))
	g.Expect(results[2].Branch).To(Equal("feature-y"))

	// With include filter.
	includeRx := regexp.MustCompile(`^feature-`)
	filteredResults, err := parseGoGitBranches(refs, filtering.Filters{Include: includeRx})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(filteredResults).To(HaveLen(2))
	g.Expect(filteredResults[0].Branch).To(Equal("feature-x"))
	g.Expect(filteredResults[1].Branch).To(Equal("feature-y"))

	// With limit.
	limitResults, err := parseGoGitBranches(refs, filtering.Filters{Limit: 2})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(limitResults).To(HaveLen(2))
}

func TestAWSCodeCommitProviderRejectsExcessiveResultLimits(t *testing.T) {
	g := NewWithT(t)
	provider := &AWSCodeCommitProvider{}
	opts := Options{Filters: filtering.Filters{Limit: maxGitProviderResults + 1}}

	_, err := provider.ListTags(t.Context(), opts)
	g.Expect(err).To(MatchError(ContainSubstring("exceeds maximum")))
	_, err = provider.ListBranches(t.Context(), opts)
	g.Expect(err).To(MatchError(ContainSubstring("exceeds maximum")))
	_, err = provider.ListRequests(t.Context(), opts)
	g.Expect(err).To(MatchError(ContainSubstring("exceeds maximum")))
}

func TestAWSCodeCommitProviderRejectsRepeatedPullRequestTokens(t *testing.T) {
	g := NewWithT(t)
	var calls int
	client := &fakeAWSCodeCommitClient{
		listPullRequests: func(_ context.Context, input *codecommit.ListPullRequestsInput) (*codecommit.ListPullRequestsOutput, error) {
			calls++
			g.Expect(input.MaxResults).NotTo(BeNil())
			g.Expect(*input.MaxResults).To(Equal(int32(gitProviderPageSize)))
			return &codecommit.ListPullRequestsOutput{NextToken: new("same")}, nil
		},
	}
	provider := &AWSCodeCommitProvider{Client: client, RepoName: "repo"}

	_, err := provider.ListRequests(t.Context(), Options{})
	g.Expect(err).To(MatchError(ContainSubstring("repeated continuation token")))
	g.Expect(calls).To(Equal(2))
}

func newConstraint(s string) *semver.Constraints {
	c, err := semver.NewConstraint(s)
	if err != nil {
		panic(err)
	}
	return c
}
