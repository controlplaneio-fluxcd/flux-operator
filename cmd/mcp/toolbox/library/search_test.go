// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"testing"
)

func buildTestIndex() *SearchIndex {
	docs := []SearchDocument{
		{
			ID:      "gitrepository",
			Content: "GitRepository defines a source for Git repositories. Authentication with SSH keys and HTTPS tokens. Configure reconciliation retry logic for git operations. The retry mechanism handles transient failures.",
			Metadata: DocumentMetadata{
				Kind:  "GitRepository",
				Group: "source.toolkit.fluxcd.io",
			},
		},
		{
			ID:      "helmrelease",
			Content: "HelmRelease defines a Helm chart release. Configure drift detection and rollback. Set retry logic for failed deployments. The retry logic can be customized with intervals and backoff strategies.",
			Metadata: DocumentMetadata{
				Kind:  "HelmRelease",
				Group: "helm.toolkit.fluxcd.io",
			},
		},
		{
			ID:      "kustomization",
			Content: "Kustomization defines a kustomize overlay. Configure health checks and retry intervals. Prune resources automatically. Retry logic helps recover from temporary errors during reconciliation.",
			Metadata: DocumentMetadata{
				Kind:  "Kustomization",
				Group: "kustomize.toolkit.fluxcd.io",
			},
		},
	}

	index := &SearchIndex{
		Documents: make([]SearchDocument, 0, len(docs)),
		Terms:     make(map[string][]Posting),
		TotalDocs: len(docs),
	}

	// Build the index
	totalLength := 0
	for docID, doc := range docs {
		tokens := Tokenize(doc.Content)
		doc.Length = len(tokens)
		totalLength += len(tokens)

		termFreq := make(map[string]int)
		for _, token := range tokens {
			termFreq[token]++
		}

		for term, freq := range termFreq {
			index.Terms[term] = append(index.Terms[term], Posting{
				DocID:     docID,
				Frequency: freq,
			})
		}

		index.Documents = append(index.Documents, doc)
	}

	index.AvgDocLength = float64(totalLength) / float64(len(docs))

	return index
}

func TestSearch_BasicQuery(t *testing.T) {
	index := buildTestIndex()

	tests := []struct {
		query         string
		limit         int
		expectedFirst string // Expected first result Kind
		minResults    int
	}{
		{
			query:         "GitRepository",
			limit:         1,
			expectedFirst: "GitRepository",
			minResults:    1,
		},
		{
			query:         "helm",
			limit:         1,
			expectedFirst: "HelmRelease",
			minResults:    1,
		},
		{
			query:         "retry logic",
			limit:         2,
			expectedFirst: "", // Multiple docs have "retry logic"
			minResults:    2,
		},
		{
			query:         "drift detection",
			limit:         1,
			expectedFirst: "HelmRelease",
			minResults:    1,
		},
		{
			query:         "authentication SSH",
			limit:         1,
			expectedFirst: "GitRepository",
			minResults:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := index.Search(tt.query, tt.limit)

			if len(results) < tt.minResults {
				t.Errorf("Search(%q, %d) returned %d results, want at least %d",
					tt.query, tt.limit, len(results), tt.minResults)
			}

			if tt.expectedFirst != "" && len(results) > 0 {
				if results[0].Document.Metadata.Kind != tt.expectedFirst {
					t.Errorf("Search(%q) first result = %s, want %s",
						tt.query, results[0].Document.Metadata.Kind, tt.expectedFirst)
				}
			}

			// Check that results are sorted by score (descending)
			for i := 1; i < len(results); i++ {
				if results[i].Score > results[i-1].Score {
					t.Errorf("Results not sorted by score: result[%d].Score=%f > result[%d].Score=%f",
						i, results[i].Score, i-1, results[i-1].Score)
				}
			}
		})
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	index := buildTestIndex()

	results := index.Search("", 5)
	if results != nil {
		t.Errorf("Search with empty query should return nil, got %d results", len(results))
	}
}

func TestSearch_NoMatches(t *testing.T) {
	index := buildTestIndex()

	results := index.Search("nonexistent xyz abc", 5)
	if results != nil {
		t.Errorf("Search with no matches should return nil, got %d results", len(results))
	}
}

func TestSearch_Limit(t *testing.T) {
	index := buildTestIndex()

	// Search for common term that appears in all docs
	results := index.Search("configure", 2)

	if len(results) > 2 {
		t.Errorf("Search with limit=2 returned %d results, want at most 2", len(results))
	}
}

func TestSearch_ScorePositive(t *testing.T) {
	index := buildTestIndex()

	results := index.Search("GitRepository authentication", 5)

	for i, result := range results {
		if result.Score <= 0 {
			t.Errorf("Result[%d] has non-positive score: %f", i, result.Score)
		}
	}
}

func TestSearch_StopWordFiltering(t *testing.T) {
	index := buildTestIndex()

	// Query with all stop words should return nothing
	results := index.Search("the and for with", 5)

	if results != nil {
		t.Errorf("Search with only stop words should return nil, got %d results", len(results))
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	index := buildTestIndex()

	// Use a term that exists in the content
	resultsLower := index.Search("authentication", 1)
	resultsUpper := index.Search("AUTHENTICATION", 1)
	resultsMixed := index.Search("Authentication", 1)

	if len(resultsLower) == 0 || len(resultsUpper) == 0 || len(resultsMixed) == 0 {
		t.Errorf("Search should be case insensitive: lower=%d, upper=%d, mixed=%d",
			len(resultsLower), len(resultsUpper), len(resultsMixed))
		return
	}

	// All queries should return the same first result
	if resultsLower[0].Document.ID != resultsUpper[0].Document.ID ||
		resultsLower[0].Document.ID != resultsMixed[0].Document.ID {
		t.Error("Case-insensitive searches should return same results")
	}
}

func TestFindCandidates(t *testing.T) {
	index := buildTestIndex()

	queryTerms := []string{"git", "authentication"}
	candidates := index.findCandidates(queryTerms)

	// Should find at least one candidate
	if len(candidates) == 0 {
		t.Error("findCandidates should find at least one document")
	}

	// Each candidate should have matched terms
	for docID, matches := range candidates {
		if len(matches) == 0 {
			t.Errorf("Candidate docID=%d has no matched terms", docID)
		}
	}
}

func TestSearchResult_MatchedTerms(t *testing.T) {
	index := buildTestIndex()

	results := index.Search("authentication retry", 5)

	for i, result := range results {
		if len(result.Matches) == 0 {
			t.Errorf("Result[%d] has no matched terms", i)
		}

		// Matched terms should be from the query
		for _, match := range result.Matches {
			if match != "authentication" && match != "retry" {
				t.Errorf("Result[%d] has unexpected matched term: %s", i, match)
			}
		}
	}
}
