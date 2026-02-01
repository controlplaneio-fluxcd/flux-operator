// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"code.gitea.io/sdk/gitea"
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

// newGiteaTestServer creates an httptest server that serves Gitea API responses.
// handlers is a map of "METHOD /path" to handler functions.
func newGiteaTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// Always serve the version endpoint so NewClient doesn't fail.
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.23.0"})
	})

	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	return httptest.NewServer(mux)
}

// newGiteaTestProvider creates a GiteaProvider backed by the given test server URL.
func newGiteaTestProvider(t *testing.T, serverURL string) *GiteaProvider {
	t.Helper()
	provider, err := NewGiteaProvider(context.Background(), Options{
		URL: serverURL + "/testowner/testrepo",
	})
	if err != nil {
		t.Fatalf("failed to create Gitea provider: %v", err)
	}
	return provider
}

func TestGiteaProvider_ListTags(t *testing.T) {
	newConstraint := func(s string) *semver.Constraints {
		c, err := semver.NewConstraint(s)
		if err != nil {
			panic(err)
		}
		return c
	}

	testTags := []*gitea.Tag{
		{Name: "6.0.4", Commit: &gitea.CommitMeta{SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
		{Name: "5.1.0", Commit: &gitea.CommitMeta{SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
		{Name: "5.0.3", Commit: &gitea.CommitMeta{SHA: "95be17be1dc2103eb5e2c0b0bac50ef692c4657d"}},
		{Name: "5.0.2", Commit: &gitea.CommitMeta{SHA: "6596ed08de58bffc6982512a0483be3b2ec346ce"}},
		{Name: "5.0.1", Commit: &gitea.CommitMeta{SHA: "7411da595c25183daba255068814b83843fe3395"}},
		{Name: "5.0.0", Commit: &gitea.CommitMeta{SHA: "9299a2d1f300267354609bee398caa2cb5548594"}},
		{Name: "4.1.0", Commit: &gitea.CommitMeta{SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009"}},
	}

	tests := []struct {
		name string
		opts Options
		want []Result
	}{
		{
			name: "filters tags by semver",
			opts: Options{
				Filters: filtering.Filters{
					SemVer: newConstraint("5.0.x"),
				},
			},
			want: []Result{
				{ID: "48562421", SHA: "95be17be1dc2103eb5e2c0b0bac50ef692c4657d", Tag: "5.0.3"},
				{ID: "48496884", SHA: "6596ed08de58bffc6982512a0483be3b2ec346ce", Tag: "5.0.2"},
				{ID: "48431347", SHA: "7411da595c25183daba255068814b83843fe3395", Tag: "5.0.1"},
				{ID: "48365810", SHA: "9299a2d1f300267354609bee398caa2cb5548594", Tag: "5.0.0"},
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
			want: []Result{
				{ID: "48955639", SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009", Tag: "6.0.4"},
			},
		},
		{
			name: "filters tags by limit",
			opts: Options{
				Filters: filtering.Filters{
					Limit: 1,
				},
			},
			want: []Result{
				{ID: "48955639", SHA: "11cf36d83818e64aaa60d523ab6438258ebb6009", Tag: "6.0.4"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			srv := newGiteaTestServer(t, map[string]http.HandlerFunc{
				"GET /api/v1/repos/testowner/testrepo/tags": func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(testTags)
				},
			})
			defer srv.Close()

			provider := newGiteaTestProvider(t, srv.URL)

			got, err := provider.ListTags(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}

func TestGiteaProvider_ListBranches(t *testing.T) {
	testBranches := []*gitea.Branch{
		{Name: "patch-1", Commit: &gitea.PayloadCommit{ID: "cebef2d870bc83b37f43c470bae205fca094bacc"}},
		{Name: "patch-2", Commit: &gitea.PayloadCommit{ID: "a275fb0322466eaa1a74485a4f79f88d7c8858e8"}},
		{Name: "random-branch", Commit: &gitea.PayloadCommit{ID: "dba7673010f19a94af4345453005933fd511bea9"}},
		{Name: "patch-3", Commit: &gitea.PayloadCommit{ID: "f2aed00334494f13d92d065ecda39aea0d0b871f"}},
		{Name: "patch-4", Commit: &gitea.PayloadCommit{ID: "4e1243bd22c66e76c2ba9eddc1f91394e57f9f83"}},
	}

	tests := []struct {
		name string
		opts Options
		want []Result
	}{
		{
			name: "filters branches by regex",
			opts: Options{
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^patch-.*`),
					Exclude: regexp.MustCompile(`^patch-4`),
				},
			},
			want: []Result{
				{ID: "183501423", SHA: "cebef2d870bc83b37f43c470bae205fca094bacc", Branch: "patch-1"},
				{ID: "183566960", SHA: "a275fb0322466eaa1a74485a4f79f88d7c8858e8", Branch: "patch-2"},
				{ID: "183632497", SHA: "f2aed00334494f13d92d065ecda39aea0d0b871f", Branch: "patch-3"},
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
			want: []Result{
				{ID: "183501423", SHA: "cebef2d870bc83b37f43c470bae205fca094bacc", Branch: "patch-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			srv := newGiteaTestServer(t, map[string]http.HandlerFunc{
				"GET /api/v1/repos/testowner/testrepo/branches": func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(testBranches)
				},
			})
			defer srv.Close()

			provider := newGiteaTestProvider(t, srv.URL)

			got, err := provider.ListBranches(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}

func TestGiteaProvider_ListRequests(t *testing.T) {
	testPRs := []*gitea.PullRequest{
		{
			Index:  5,
			Head:   &gitea.PRBranchInfo{Ref: "feat/5", Sha: "3fd0d45b23e5f14089587a9049e33d82497b944b"},
			Title:  "test5: Edit README.md",
			Poster: &gitea.User{UserName: "stefanprodan"},
			Labels: []*gitea.Label{},
		},
		{
			Index:  4,
			Head:   &gitea.PRBranchInfo{Ref: "patch-4", Sha: "a143f78b7f8abd511a4f4ce84b4875edfb621a56"},
			Title:  "test4: Edit README.md",
			Poster: &gitea.User{UserName: "stefanprodan"},
			Labels: []*gitea.Label{{Name: "documentation"}, {Name: "enhancement"}},
		},
		{
			Index:  3,
			Head:   &gitea.PRBranchInfo{Ref: "patch-3", Sha: "f2aed00334494f13d92d065ecda39aea0d0b871f"},
			Title:  "test3: Edit README.md",
			Poster: &gitea.User{UserName: "stefanprodan"},
			Labels: []*gitea.Label{{Name: "documentation"}},
		},
		{
			Index:  2,
			Head:   &gitea.PRBranchInfo{Ref: "patch-2", Sha: "a275fb0322466eaa1a74485a4f79f88d7c8858e8"},
			Title:  "test2: Edit README.md",
			Poster: &gitea.User{UserName: "stefanprodan"},
			Labels: []*gitea.Label{{Name: "enhancement"}},
		},
		{
			Index:  1,
			Head:   &gitea.PRBranchInfo{Ref: "patch-1", Sha: "cebef2d870bc83b37f43c470bae205fca094bacc"},
			Title:  "test1: Edit README.md",
			Poster: &gitea.User{UserName: "stefanprodan"},
			Labels: []*gitea.Label{{Name: "enhancement"}},
		},
	}

	tests := []struct {
		name string
		opts Options
		want []Result
	}{
		{
			name: "all pull requests",
			opts: Options{},
			want: []Result{
				{ID: "5", SHA: "3fd0d45b23e5f14089587a9049e33d82497b944b", Author: "stefanprodan", Title: "test5: Edit README.md", Branch: "feat/5", Labels: []string{}},
				{ID: "4", SHA: "a143f78b7f8abd511a4f4ce84b4875edfb621a56", Author: "stefanprodan", Title: "test4: Edit README.md", Branch: "patch-4", Labels: []string{"documentation", "enhancement"}},
				{ID: "3", SHA: "f2aed00334494f13d92d065ecda39aea0d0b871f", Author: "stefanprodan", Title: "test3: Edit README.md", Branch: "patch-3", Labels: []string{"documentation"}},
				{ID: "2", SHA: "a275fb0322466eaa1a74485a4f79f88d7c8858e8", Author: "stefanprodan", Title: "test2: Edit README.md", Branch: "patch-2", Labels: []string{"enhancement"}},
				{ID: "1", SHA: "cebef2d870bc83b37f43c470bae205fca094bacc", Author: "stefanprodan", Title: "test1: Edit README.md", Branch: "patch-1", Labels: []string{"enhancement"}},
			},
		},
		{
			name: "filters pull requests by labels and limit",
			opts: Options{
				Filters: filtering.Filters{
					Limit:  2,
					Labels: []string{"enhancement"},
				},
			},
			want: []Result{
				{ID: "4", SHA: "a143f78b7f8abd511a4f4ce84b4875edfb621a56", Author: "stefanprodan", Title: "test4: Edit README.md", Branch: "patch-4", Labels: []string{"documentation", "enhancement"}},
				{ID: "2", SHA: "a275fb0322466eaa1a74485a4f79f88d7c8858e8", Author: "stefanprodan", Title: "test2: Edit README.md", Branch: "patch-2", Labels: []string{"enhancement"}},
			},
		},
		{
			name: "filters pull requests by branch regex",
			opts: Options{
				Filters: filtering.Filters{
					Include: regexp.MustCompile(`^feat/.*`),
				},
			},
			want: []Result{
				{ID: "5", SHA: "3fd0d45b23e5f14089587a9049e33d82497b944b", Author: "stefanprodan", Title: "test5: Edit README.md", Branch: "feat/5", Labels: []string{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			srv := newGiteaTestServer(t, map[string]http.HandlerFunc{
				"GET /api/v1/repos/testowner/testrepo/pulls": func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(testPRs)
				},
			})
			defer srv.Close()

			provider := newGiteaTestProvider(t, srv.URL)

			got, err := provider.ListRequests(context.Background(), tt.opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(got).To(BeEquivalentTo(tt.want))
		})
	}
}

func TestGiteaProvider_ListEnvironments(t *testing.T) {
	g := NewWithT(t)

	srv := newGiteaTestServer(t, nil)
	defer srv.Close()

	provider := newGiteaTestProvider(t, srv.URL)

	_, err := provider.ListEnvironments(context.Background(), Options{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not supported"))
}

func TestGiteaProvider_ParseURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid http URL",
			url:       "http://localhost:3000/owner/repo",
			wantHost:  "http://localhost:3000",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "valid https URL",
			url:       "https://gitea.example.com/org/project",
			wantHost:  "https://gitea.example.com",
			wantOwner: "org",
			wantRepo:  "project",
		},
		{
			name:    "missing repo",
			url:     "http://localhost:3000/owner",
			wantErr: true,
		},
		{
			name:    "too many path segments",
			url:     "http://localhost:3000/a/b/c",
			wantErr: true,
		},
		{
			name:    "empty path",
			url:     "http://localhost:3000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			host, owner, repo, err := parseGiteaURL(tt.url)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(host).To(Equal(tt.wantHost))
			g.Expect(owner).To(Equal(tt.wantOwner))
			g.Expect(repo).To(Equal(tt.wantRepo))
		})
	}
}
