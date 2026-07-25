// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

// Git provider safety limits bound pagination, response decoding, and retained results.
const (
	gitProviderPageSize            = 100
	maxGitProviderPages            = 1000
	maxGitProviderResponseBodySize = 10 * 1024 * 1024
	maxGitProviderResults          = 10_000
)

// gitProviderResultLimit returns the effective result limit after applying the
// default and validating the configured maximum.
func gitProviderResultLimit(filters filtering.Filters) (int, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = filtering.DefaultLimit
	}
	if limit > maxGitProviderResults {
		return 0, fmt.Errorf("result limit %d exceeds maximum of %d", limit, maxGitProviderResults)
	}
	return limit, nil
}

// responseLimitRoundTripper limits decompressed response bodies before an API
// client decodes them.
type responseLimitRoundTripper struct {
	base     http.RoundTripper
	maxBytes int64
}

// RoundTrip limits the response body returned by the underlying transport.
func (t *responseLimitRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		resp.Body = http.MaxBytesReader(nil, resp.Body, t.maxBytes)
	}
	return resp, nil
}

// newGitProviderHTTPClient returns an HTTP client with standard transport
// defaults, optional custom TLS configuration, and bounded response bodies.
func newGitProviderHTTPClient(tlsConfig *tls.Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	return &http.Client{Transport: &responseLimitRoundTripper{
		base:     transport,
		maxBytes: maxGitProviderResponseBodySize,
	}}
}

// paginationGuard bounds remote pagination and detects repeated pages.
type paginationGuard struct {
	operation string
	maxPages  int
	pages     int
	seen      map[int]struct{}
}

// newPaginationGuard creates a page-number guard with the given maximum.
func newPaginationGuard(operation string, maxPages int) *paginationGuard {
	return &paginationGuard{
		operation: operation,
		maxPages:  maxPages,
		seen:      make(map[int]struct{}, maxPages),
	}
}

// newGitProviderPaginationGuard creates a page-number guard using the standard maximum.
func newGitProviderPaginationGuard(operation string) *paginationGuard {
	return newPaginationGuard(operation, maxGitProviderPages)
}

// Visit records a page before it is requested.
func (g *paginationGuard) Visit(page int) error {
	if g.pages >= g.maxPages {
		return fmt.Errorf("%s pagination exceeds maximum of %d pages", g.operation, g.maxPages)
	}
	if _, ok := g.seen[page]; ok {
		return fmt.Errorf("%s pagination returned repeated page %d", g.operation, page)
	}
	g.seen[page] = struct{}{}
	g.pages++
	return nil
}

// paginationTokenGuard bounds token-based pagination and detects repeated tokens.
type paginationTokenGuard struct {
	operation string
	maxPages  int
	pages     int
	seen      map[string]struct{}
}

// newGitProviderPaginationTokenGuard creates a token guard using the standard maximum.
func newGitProviderPaginationTokenGuard(operation string) *paginationTokenGuard {
	return &paginationTokenGuard{
		operation: operation,
		maxPages:  maxGitProviderPages,
		seen:      make(map[string]struct{}, maxGitProviderPages),
	}
}

// Visit records a continuation token before its page is requested.
func (g *paginationTokenGuard) Visit(token string) error {
	if g.pages >= g.maxPages {
		return fmt.Errorf("%s pagination exceeds maximum of %d pages", g.operation, g.maxPages)
	}
	if _, ok := g.seen[token]; ok {
		return fmt.Errorf("%s pagination returned a repeated continuation token", g.operation)
	}
	g.seen[token] = struct{}{}
	g.pages++
	return nil
}
