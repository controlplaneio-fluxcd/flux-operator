// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"net/url"
	"testing"
)

func TestIsSafeRedirectPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "valid root path",
			path: "/",
			want: true,
		},
		{
			name: "valid simple path",
			path: "/dashboard",
			want: true,
		},
		{
			name: "valid path with query",
			path: "/resource?name=test",
			want: true,
		},
		{
			name: "valid nested path",
			path: "/api/v1/resources",
			want: true,
		},
		{
			name: "protocol-relative URL blocked",
			path: "//evil.com",
			want: false,
		},
		{
			name: "protocol-relative URL with path blocked",
			path: "//evil.com/phishing",
			want: false,
		},
		{
			name: "absolute URL with http blocked",
			path: "http://evil.com",
			want: false,
		},
		{
			name: "absolute URL with https blocked",
			path: "https://evil.com",
			want: false,
		},
		{
			name: "absolute URL with https and path blocked",
			path: "https://evil.com/phishing",
			want: false,
		},
		{
			name: "javascript scheme blocked",
			path: "javascript://alert(1)",
			want: false,
		},
		{
			name: "data scheme blocked",
			path: "data://text/html,<script>alert(1)</script>",
			want: false,
		},
		{
			name: "relative path without leading slash blocked",
			path: "dashboard",
			want: false,
		},
		{
			name: "empty path blocked",
			path: "",
			want: false,
		},
		{
			name: "path with embedded scheme blocked",
			path: "/redirect?url=https://evil.com",
			want: true, // This is fine, the scheme is in the query not the path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeRedirectPath(tt.path); got != tt.want {
				t.Errorf("isSafeRedirectPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestOriginalURL(t *testing.T) {
	tests := []struct {
		name     string
		query    url.Values
		expected string
	}{
		{
			name:     "no original path defaults to root",
			query:    url.Values{},
			expected: "/",
		},
		{
			name:     "valid original path",
			query:    url.Values{authQueryParamOriginalPath: []string{"/dashboard"}},
			expected: "/dashboard",
		},
		{
			name:     "malicious absolute URL blocked",
			query:    url.Values{authQueryParamOriginalPath: []string{"https://evil.com"}},
			expected: "/",
		},
		{
			name:     "malicious protocol-relative URL blocked",
			query:    url.Values{authQueryParamOriginalPath: []string{"//evil.com"}},
			expected: "/",
		},
		{
			name:     "preserves other query params",
			query:    url.Values{authQueryParamOriginalPath: []string{"/dashboard"}, "foo": []string{"bar"}},
			expected: "/dashboard?foo=bar",
		},
		{
			name:     "malicious URL with preserved query params",
			query:    url.Values{authQueryParamOriginalPath: []string{"https://evil.com"}, "foo": []string{"bar"}},
			expected: "/?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy since originalURL modifies the query
			query := make(url.Values)
			for k, v := range tt.query {
				query[k] = v
			}
			if got := originalURL(query); got != tt.expected {
				t.Errorf("originalURL(%v) = %q, want %q", tt.query, got, tt.expected)
			}
		})
	}
}
