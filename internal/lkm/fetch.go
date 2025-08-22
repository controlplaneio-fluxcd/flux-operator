// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// fetchOptions holds the internal configuration for the Fetch function.
type fetchOptions struct {
	retries            int
	allowLocalhost     bool
	userAgent          string
	insecureSkipVerify bool
	contentType        string
}

// FetchOption configures a Fetch operation.
type FetchOption func(*fetchOptions)

// FetchOpt contains options for the Fetch function.
var FetchOpt fetchOptionBuilder

// fetchOptionBuilder is the internal builder for FetchOption functions.
type fetchOptionBuilder struct{}

// WithContentType sets the expected Content-Type header for HTTP requests.
func (fetchOptionBuilder) WithContentType(contentType string) FetchOption {
	return func(opts *fetchOptions) {
		opts.contentType = contentType
	}
}

// WithRetries sets the number of retries for HTTP requests.
func (fetchOptionBuilder) WithRetries(retries int) FetchOption {
	return func(opts *fetchOptions) {
		opts.retries = retries
	}
}

// WithLocalhost allows HTTP connections to localhost addresses.
func (fetchOptionBuilder) WithLocalhost(allow bool) FetchOption {
	return func(opts *fetchOptions) {
		opts.allowLocalhost = allow
	}
}

// WithUserAgent sets the User-Agent header for HTTP requests.
func (fetchOptionBuilder) WithUserAgent(userAgent string) FetchOption {
	return func(opts *fetchOptions) {
		opts.userAgent = userAgent
	}
}

// WithInsecureSkipVerify skips TLS certificate verification (for testing).
func (fetchOptionBuilder) WithInsecureSkipVerify(skip bool) FetchOption {
	return func(opts *fetchOptions) {
		opts.insecureSkipVerify = skip
	}
}

// Fetch performs an HTTP GET request to the specified URL.
// It enforces HTTPS unless connecting to localhost and allows various options
// to customize the request behavior.
func Fetch(ctx context.Context, rawURL string, opts ...FetchOption) ([]byte, error) {
	// Configure default options.
	options := &fetchOptions{
		retries:        2,
		userAgent:      "flux-operator-lkm/1.0",
		allowLocalhost: true,
	}

	// Apply user-provided options.
	for _, opt := range opts {
		opt(options)
	}

	// Parse and validate the URL.
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Check if the hostname is localhost or equivalent.
	isLocalhost := strings.EqualFold(parsedURL.Hostname(), "localhost") ||
		parsedURL.Hostname() == "127.0.0.1" ||
		parsedURL.Hostname() == "::1"

	// Enforce HTTPS unless connecting to localhost and allowed.
	if !strings.EqualFold(parsedURL.Scheme, "https") && (!isLocalhost || !options.allowLocalhost) {
		return nil, errors.New("URL must use HTTPS scheme")
	}

	// Set up the retryable HTTP client.
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = options.retries
	retryClient.RetryWaitMin = 2 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.Logger = nil
	if options.insecureSkipVerify {
		retryClient.HTTPClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Create the HTTP request with context.
	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", options.userAgent)
	if options.contentType != "" {
		req.Header.Set("Content-Type", options.contentType)
	}

	// Perform the HTTP GET request.
	resp, err := retryClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for successful response.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch failed with status: %d", resp.StatusCode)
	}

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Ensure the body is not empty.
	if len(body) == 0 {
		return nil, errors.New("response body is empty")
	}

	if strings.EqualFold(options.contentType, "application/json") && !json.Valid(body) {
		return nil, errors.New("invalid JSON response")
	}

	return body, nil
}
