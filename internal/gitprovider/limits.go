// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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
		resp.Body = &responseLimitReader{
			ReadCloser: http.MaxBytesReader(nil, resp.Body, t.maxBytes),
			maxBytes:   t.maxBytes,
		}
	}
	return resp, nil
}

// responseLimitReader wraps a size-limited response body so that the limiter's
// request-oriented error surfaces the response size cap instead.
type responseLimitReader struct {
	io.ReadCloser
	maxBytes int64
}

// Read prefixes the http.MaxBytesReader error with the response body limit.
func (r *responseLimitReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		err = fmt.Errorf("response body exceeds the maximum allowed size of %d bytes: %w", r.maxBytes, err)
	}
	return n, err
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

// paginationGuard bounds remote pagination and detects repeated page numbers
// or continuation tokens.
type paginationGuard[T comparable] struct {
	operation string
	keyKind   string
	maxPages  int
	pages     int
	seen      map[T]struct{}
}

// newPaginationGuard creates a page-number guard with the given maximum.
func newPaginationGuard(operation string, maxPages int) *paginationGuard[int] {
	return &paginationGuard[int]{
		operation: operation,
		keyKind:   "page",
		maxPages:  maxPages,
		seen:      make(map[int]struct{}, maxPages),
	}
}

// newGitProviderPaginationGuard creates a page-number guard using the standard maximum.
func newGitProviderPaginationGuard(operation string) *paginationGuard[int] {
	return newPaginationGuard(operation, maxGitProviderPages)
}

// newGitProviderPaginationTokenGuard creates a continuation-token guard using the standard maximum.
func newGitProviderPaginationTokenGuard(operation string) *paginationGuard[string] {
	return &paginationGuard[string]{
		operation: operation,
		keyKind:   "continuation token",
		maxPages:  maxGitProviderPages,
		seen:      make(map[string]struct{}, maxGitProviderPages),
	}
}

// Visit records a page number or continuation token before its page is requested.
func (g *paginationGuard[T]) Visit(key T) error {
	if g.pages >= g.maxPages {
		return fmt.Errorf("%s pagination exceeds maximum of %d pages", g.operation, g.maxPages)
	}
	if _, ok := g.seen[key]; ok {
		return fmt.Errorf("%s pagination returned a repeated %s", g.operation, g.keyKind)
	}
	g.seen[key] = struct{}{}
	g.pages++
	return nil
}
