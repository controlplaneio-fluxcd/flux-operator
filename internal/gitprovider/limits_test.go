// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v87/github"
	. "github.com/onsi/gomega"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

func TestTagSelectorBoundsRetainedResults(t *testing.T) {
	g := NewWithT(t)
	selector, err := newTagSelector(filtering.Filters{Limit: 10})
	g.Expect(err).NotTo(HaveOccurred())

	for i := 0; i < 5000; i++ {
		tag := fmt.Sprintf("v%05d", i)
		selector.Add(tag, Result{Tag: tag})
	}

	results := selector.Results()
	g.Expect(results).To(HaveLen(10))
	g.Expect(results[0].Tag).To(Equal("v04999"))
	g.Expect(results[9].Tag).To(Equal("v04990"))
}

func TestTagSelectorPreservesSemverOrdering(t *testing.T) {
	g := NewWithT(t)
	constraint, err := semver.NewConstraint(">= 0.0.0")
	g.Expect(err).NotTo(HaveOccurred())
	selector, err := newTagSelector(filtering.Filters{Limit: 3, SemVer: constraint})
	g.Expect(err).NotTo(HaveOccurred())

	for _, tag := range []string{"v2.0.0", "invalid", "v1.0.0", "v10.0.0", "v3.0.0"} {
		selector.Add(tag, Result{Tag: tag})
	}

	results := selector.Results()
	g.Expect(results).To(HaveLen(3))
	g.Expect([]string{results[0].Tag, results[1].Tag, results[2].Tag}).To(Equal(
		[]string{"v10.0.0", "v3.0.0", "v2.0.0"}))
}

func TestPaginationGuard(t *testing.T) {
	t.Run("rejects repeated pages", func(t *testing.T) {
		g := NewWithT(t)
		guard := newPaginationGuard("tags", 100)
		g.Expect(guard.Visit(0)).To(Succeed())
		g.Expect(guard.Visit(2)).To(Succeed())
		g.Expect(guard.Visit(2)).To(MatchError(ContainSubstring("repeated page")))
	})

	t.Run("rejects pages over maximum", func(t *testing.T) {
		g := NewWithT(t)
		guard := newPaginationGuard("tags", 2)
		g.Expect(guard.Visit(0)).To(Succeed())
		g.Expect(guard.Visit(2)).To(Succeed())
		g.Expect(guard.Visit(3)).To(MatchError(ContainSubstring("maximum of 2 pages")))
	})
}

func TestPaginationTokenGuard(t *testing.T) {
	g := NewWithT(t)
	guard := newGitProviderPaginationTokenGuard("pull requests")
	g.Expect(guard.Visit("")).To(Succeed())
	g.Expect(guard.Visit("next")).To(Succeed())
	g.Expect(guard.Visit("next")).To(MatchError(ContainSubstring("repeated continuation token")))
}

func TestGitProviderResponseBodyLimit(t *testing.T) {
	g := NewWithT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, maxGitProviderResponseBodySize+1))
	}))
	defer srv.Close()

	resp, err := newGitProviderHTTPClient(nil).Get(srv.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	var maxBytesErr *http.MaxBytesError
	g.Expect(errors.As(err, &maxBytesErr)).To(BeTrue())
	g.Expect(maxBytesErr.Limit).To(Equal(int64(maxGitProviderResponseBodySize)))
	g.Expect(err.Error()).To(ContainSubstring("response body exceeds the maximum allowed size"))
}

func TestProvidersRejectRepeatedTagPages(t *testing.T) {
	t.Run("GitHub", func(t *testing.T) {
		g := NewWithT(t)
		var requests atomic.Int32
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Link", fmt.Sprintf("<%s/repos/owner/repo/tags?page=1>; rel=\"next\"", srv.URL))
			_, _ = io.WriteString(w, `[{"name":"v1.0.0","commit":{"sha":"abc"}}]`)
		}))
		defer srv.Close()

		client, err := github.NewClient(
			github.WithHTTPClient(newGitProviderHTTPClient(nil)),
			github.WithEnterpriseURLs(srv.URL, srv.URL),
		)
		g.Expect(err).NotTo(HaveOccurred())
		provider := &GitHubProvider{Client: client, Owner: "owner", Repo: "repo"}

		_, err = provider.ListTags(context.Background(), Options{})
		g.Expect(err).To(MatchError(ContainSubstring("repeated page")))
		g.Expect(requests.Load()).To(Equal(int32(2)))
	})

	t.Run("GitLab", func(t *testing.T) {
		g := NewWithT(t)
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Next-Page", "1")
			_, _ = io.WriteString(w, `[{"name":"v1.0.0","commit":{"id":"abc"}}]`)
		}))
		defer srv.Close()

		client, err := gitlab.NewClient("",
			gitlab.WithBaseURL(srv.URL+"/"),
			gitlab.WithHTTPClient(newGitProviderHTTPClient(nil)))
		g.Expect(err).NotTo(HaveOccurred())
		provider := &GitLabProvider{Client: client, Project: "owner/repo"}

		_, err = provider.ListTags(context.Background(), Options{})
		g.Expect(err).To(MatchError(ContainSubstring("repeated page")))
		g.Expect(requests.Load()).To(Equal(int32(2)))
	})

	t.Run("Gitea", func(t *testing.T) {
		g := NewWithT(t)
		var requests atomic.Int32
		var srv *httptest.Server
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/version" {
				_, _ = io.WriteString(w, `{"version":"1.23.0"}`)
				return
			}
			requests.Add(1)
			w.Header().Set("Link", fmt.Sprintf("<%s/api/v1/repos/owner/repo/tags?page=1>; rel=\"next\"", srv.URL))
			_, _ = io.WriteString(w, `[{"name":"v1.0.0","commit":{"sha":"abc"}}]`)
		}))
		defer srv.Close()

		provider, err := NewGiteaProvider(context.Background(), Options{URL: srv.URL + "/owner/repo"})
		g.Expect(err).NotTo(HaveOccurred())
		_, err = provider.ListTags(context.Background(), Options{})
		g.Expect(err).To(MatchError(ContainSubstring("repeated page")))
		g.Expect(requests.Load()).To(Equal(int32(2)))
	})
}

func TestGitLabProviderHonorsCanceledContext(t *testing.T) {
	g := NewWithT(t)
	requestStarted := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted <- struct{}{}
		<-r.Context().Done()
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	client, err := gitlab.NewClient("",
		gitlab.WithBaseURL(srv.URL+"/"),
		gitlab.WithHTTPClient(newGitProviderHTTPClient(nil)))
	g.Expect(err).NotTo(HaveOccurred())
	provider := &GitLabProvider{Client: client, Project: "owner/repo"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := provider.ListTags(ctx, Options{})
		done <- err
	}()

	<-requestStarted
	cancel()
	g.Eventually(done).Should(Receive(MatchError(ContainSubstring("context canceled"))))
}

func TestTagSelectorMatchesCanonicalTieAndDuplicateOrdering(t *testing.T) {
	constraint, err := semver.NewConstraint(">= 0.0.0")
	NewWithT(t).Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name string
		tags []string
	}{
		{name: "prefix variants", tags: []string{"1.0.0", "v1.0.0"}},
		{name: "build metadata", tags: []string{"1.0.0+a", "1.0.0+z"}},
		{name: "zero padding", tags: []string{"1.2.3", "v01.02.03"}},
		{name: "duplicates", tags: []string{"v2.0.0", "v2.0.0", "v1.0.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			filters := filtering.Filters{Limit: len(tt.tags), SemVer: constraint}
			selector, err := newTagSelector(filters)
			g.Expect(err).NotTo(HaveOccurred())
			for i, tag := range tt.tags {
				selector.Add(tag, Result{Tag: tag, SHA: fmt.Sprintf("sha-%d", i)})
			}

			results := selector.Results()
			got := make([]string, len(results))
			for i := range results {
				got[i] = results[i].Tag
			}
			g.Expect(got).To(Equal(filters.Tags(tt.tags)))
			if tt.name == "duplicates" {
				g.Expect(results[0].SHA).To(Equal("sha-1"))
				g.Expect(results[1].SHA).To(Equal("sha-1"))
			}
		})
	}
}

func TestGitProviderCompressedResponseBodyLimit(t *testing.T) {
	g := NewWithT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		_, _ = zw.Write(make([]byte, maxGitProviderResponseBodySize+1))
		_ = zw.Close()
	}))
	defer srv.Close()

	resp, err := newGitProviderHTTPClient(nil).Get(srv.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	var maxBytesErr *http.MaxBytesError
	g.Expect(errors.As(err, &maxBytesErr)).To(BeTrue())
}
