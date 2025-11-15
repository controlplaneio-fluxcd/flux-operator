// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"reflect"
	"sort"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple text",
			input:    "Hello World",
			expected: []string{"hello", "world"},
		},
		{
			name:     "with stop words",
			input:    "The quick brown fox",
			expected: []string{"quick", "brown", "fox"},
		},
		{
			name:     "CamelCase splitting",
			input:    "GitRepository HelmRelease",
			expected: []string{"git", "repository", "helm", "release"},
		},
		{
			name:     "version preservation",
			input:    "v1 v2beta3 api",
			expected: []string{"v1", "v2beta3", "api"},
		},
		{
			name:     "hyphenated words",
			input:    "kustomize-controller source-controller",
			expected: []string{"kustomize-controller", "source-controller"},
		},
		{
			name:     "Flux plurals stemming",
			input:    "GitRepositories Kustomizations HelmReleases",
			expected: []string{"git", "repository", "kustomization", "helm", "release"},
		},
		{
			name:     "mixed case with punctuation",
			input:    "How to configure retry logic for HelmReleases?",
			expected: []string{"configure", "retry", "logic", "helm", "release"},
		},
		{
			name:     "authentication keywords",
			input:    "SSH authentication with private keys",
			expected: []string{"ssh", "authentication", "private", "key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Tokenize(tt.input)

			// Sort both slices for comparison (order doesn't matter)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStem(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"repositories", "repository"},
		{"kustomizations", "kustomization"},
		{"helmreleases", "helmrelease"},
		{"helmrepositories", "helmrepository"},
		{"gitrepositories", "gitrepository"},
		{"reconciliations", "reconciliation"},
		{"configurations", "configuration"},
		{"authentications", "authentication"},
		{"policies", "policy"},
		{"releases", "release"},
		{"fixes", "fix"},
		{"pushes", "push"},
		{"patches", "patch"},
		// Words that shouldn't be stemmed
		{"git", "git"},
		{"flux", "flux"},
		{"helm", "helm"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stem(tt.input)
			if result != tt.expected {
				t.Errorf("stem(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"v1", true},
		{"v2", true},
		{"v2beta3", true},
		{"v1alpha1", true},
		{"version", false},
		{"api", false},
		{"", false},
		{"1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isVersion(tt.input)
			if result != tt.expected {
				t.Errorf("isVersion(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
