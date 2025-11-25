// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestHasWildcard(t *testing.T) {
	for _, tt := range []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "empty pattern",
			pattern:  "",
			expected: false,
		},
		{
			name:     "no wildcard",
			pattern:  "test",
			expected: false,
		},
		{
			name:     "single wildcard",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "wildcard at start",
			pattern:  "*test",
			expected: true,
		},
		{
			name:     "wildcard at end",
			pattern:  "test*",
			expected: true,
		},
		{
			name:     "wildcard in middle",
			pattern:  "te*st",
			expected: true,
		},
		{
			name:     "multiple wildcards",
			pattern:  "*test*",
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := hasWildcard(tt.pattern)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestMatchesWildcard(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    string
		pattern  string
		expected bool
	}{
		// Exact matching (no wildcards)
		{
			name:     "exact match",
			input:    "test",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact match case insensitive",
			input:    "Test",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact match uppercase",
			input:    "TEST",
			pattern:  "test",
			expected: true,
		},
		{
			name:     "exact no match",
			input:    "test",
			pattern:  "other",
			expected: false,
		},
		{
			name:     "empty pattern matches empty input",
			input:    "",
			pattern:  "",
			expected: true,
		},

		// Single wildcard
		{
			name:     "wildcard matches everything",
			input:    "anything",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "wildcard matches empty",
			input:    "",
			pattern:  "*",
			expected: true,
		},

		// Prefix matching
		{
			name:     "prefix match",
			input:    "test-service",
			pattern:  "test*",
			expected: true,
		},
		{
			name:     "prefix no match",
			input:    "other-service",
			pattern:  "test*",
			expected: false,
		},
		{
			name:     "prefix match case insensitive",
			input:    "Test-Service",
			pattern:  "test*",
			expected: true,
		},

		// Suffix matching
		{
			name:     "suffix match",
			input:    "my-test",
			pattern:  "*test",
			expected: true,
		},
		{
			name:     "suffix no match",
			input:    "my-other",
			pattern:  "*test",
			expected: false,
		},
		{
			name:     "suffix match case insensitive",
			input:    "my-Test",
			pattern:  "*test",
			expected: true,
		},

		// Contains matching
		{
			name:     "contains match",
			input:    "prefix-test-suffix",
			pattern:  "*test*",
			expected: true,
		},
		{
			name:     "contains no match",
			input:    "prefix-other-suffix",
			pattern:  "*test*",
			expected: false,
		},
		{
			name:     "contains match case insensitive",
			input:    "prefix-Test-suffix",
			pattern:  "*test*",
			expected: true,
		},

		// Middle wildcard
		{
			name:     "middle wildcard match",
			input:    "test-anything-service",
			pattern:  "test*service",
			expected: true,
		},
		{
			name:     "middle wildcard no match prefix",
			input:    "other-anything-service",
			pattern:  "test*service",
			expected: false,
		},
		{
			name:     "middle wildcard no match suffix",
			input:    "test-anything-other",
			pattern:  "test*service",
			expected: false,
		},
		{
			name:     "middle wildcard match case insensitive",
			input:    "Test-Anything-Service",
			pattern:  "test*service",
			expected: true,
		},

		// Multiple wildcards
		{
			name:     "multiple wildcards match",
			input:    "flux-test-my-service",
			pattern:  "*test*service*",
			expected: true,
		},
		{
			name:     "multiple wildcards no match",
			input:    "flux-other-my-deployment",
			pattern:  "*test*service*",
			expected: false,
		},
		{
			name:     "multiple wildcards adjacent",
			input:    "test",
			pattern:  "**test**",
			expected: true,
		},

		// Edge cases
		{
			name:     "wildcard with empty segments",
			input:    "test",
			pattern:  "***test***",
			expected: true,
		},
		{
			name:     "pattern longer than input",
			input:    "test",
			pattern:  "testservice",
			expected: false,
		},
		{
			name:     "input longer than pattern",
			input:    "testservice",
			pattern:  "test",
			expected: false,
		},
		{
			name:     "wildcard at start requires match at end",
			input:    "my-test-service",
			pattern:  "*test",
			expected: false,
		},
		{
			name:     "wildcard at end requires match at start",
			input:    "my-test-service",
			pattern:  "test*",
			expected: false,
		},
		{
			name:     "complex pattern match",
			input:    "flux-system-notification-controller",
			pattern:  "flux*notification*",
			expected: true,
		},
		{
			name:     "complex pattern no match",
			input:    "flux-system-source-controller",
			pattern:  "flux*notification*",
			expected: false,
		},

		// Real-world examples
		{
			name:     "flux resource prefix",
			input:    "flux-system",
			pattern:  "flux*",
			expected: true,
		},
		{
			name:     "controller suffix",
			input:    "kustomize-controller",
			pattern:  "*controller",
			expected: true,
		},
		{
			name:     "partial name search",
			input:    "my-app-deployment",
			pattern:  "*app*",
			expected: true,
		},
		{
			name:     "hyphenated names",
			input:    "flux-system-kustomize-controller",
			pattern:  "*kustomize*",
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := matchesWildcard(tt.input, tt.pattern)
			g.Expect(result).To(Equal(tt.expected), "matchesWildcard(%q, %q)", tt.input, tt.pattern)
		})
	}
}
