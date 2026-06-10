// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import "testing"

func TestConciseSearchIndexRelevance(t *testing.T) {
	index := GetSearchIndex(IndexFormatConcise)
	if index == nil {
		t.Fatal("concise search index is not available")
	}

	tests := []struct {
		query     string
		wantTitle string
	}{
		{query: "HelmRelease valuesFrom", wantTitle: "HelmRelease Reference"},
		{query: "Kustomization substituteFrom", wantTitle: "Kustomization Reference"},
		{query: "GitRepository authentication", wantTitle: "Sources Reference"},
		{query: "OCIRepository cosign verification", wantTitle: "Sources Reference"},
		{query: "ExternalArtifact", wantTitle: "Sources Reference"},
		{query: "ResourceSetInputProvider GitHub pull request", wantTitle: "ResourceSet Reference"},
		{query: "FluxInstance distribution components", wantTitle: "Flux Operator Reference"},
		{query: "ImagePolicy semver", wantTitle: "Image Automation Reference"},
		{query: "Receiver webhook hmac", wantTitle: "Notifications Reference"},
		{query: "least privilege rbac web UI", wantTitle: "Flux Web UI Reference"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := index.Search(tt.query, 1)
			if len(results) == 0 {
				t.Fatalf("Search(%q) returned no results", tt.query)
			}
			if got := results[0].Document.Metadata.Title; got != tt.wantTitle {
				t.Fatalf("Search(%q) first result = %q, want %q", tt.query, got, tt.wantTitle)
			}
		})
	}
}

func TestCompleteSearchIndexRelevance(t *testing.T) {
	index := GetSearchIndex(IndexFormatComplete)
	if index == nil {
		t.Fatal("complete search index is not available")
	}

	tests := []struct {
		query    string
		wantKind string
	}{
		{query: "GitRepository authentication", wantKind: "GitRepository"},
		{query: "ExternalArtifact", wantKind: "ExternalArtifact"},
		{query: "HelmRelease valuesFrom", wantKind: "HelmRelease"},
		{query: "Kustomization substituteFrom", wantKind: "Kustomization"},
		{query: "ResourceSetInputProvider GitHub pull request", wantKind: "ResourceSetInputProvider"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := index.Search(tt.query, 1)
			if len(results) == 0 {
				t.Fatalf("Search(%q) returned no results", tt.query)
			}
			if got := results[0].Document.Metadata.Kind; got != tt.wantKind {
				t.Fatalf("Search(%q) first result = %q, want %q", tt.query, got, tt.wantKind)
			}
		})
	}
}
