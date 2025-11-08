// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import "sort"

// SearchResult represents a single search result with score.
type SearchResult struct {
	Document SearchDocument
	Score    float64
	Matches  []string // Query terms that matched
}

// Search executes a search query and returns top-k results ranked by BM25 score + keyword boosting.
func (idx *SearchIndex) Search(query string, limit int) []SearchResult {
	// 1. Tokenize query
	queryTerms := Tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	// 2. Find candidate documents (any term match)
	candidates := idx.findCandidates(queryTerms)
	if len(candidates) == 0 {
		return nil
	}

	// 3. Score each candidate with BM25 + keyword boosting
	results := make([]SearchResult, 0, len(candidates))
	for docID := range candidates {
		// Base BM25 score on full content
		bm25Score := idx.Score(queryTerms, docID)

		// Keyword boost: count how many query terms match document keywords
		keywordBoost := idx.keywordScore(queryTerms, docID)

		// Combined score: BM25 + keyword boost
		// Keyword matches are heavily weighted to ensure document type relevance
		finalScore := bm25Score + (keywordBoost * 5.0)

		// Note: BM25 scores can be negative for terms appearing in all documents
		// We include all candidates since they matched at least one query term
		results = append(results, SearchResult{
			Document: idx.Documents[docID],
			Score:    finalScore,
			Matches:  candidates[docID],
		})
	}

	// 4. Sort by score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 5. Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// keywordScore returns the number of query terms that match document keywords.
// This provides a document-type relevance signal independent of content frequency.
func (idx *SearchIndex) keywordScore(queryTerms []string, docID int) float64 {
	doc := idx.Documents[docID]

	// Tokenize keywords to match against stemmed query terms
	keywordSet := make(map[string]bool)
	for _, kw := range doc.Metadata.Keywords {
		tokens := Tokenize(kw)
		for _, token := range tokens {
			keywordSet[token] = true
		}
	}

	// Count matching query terms
	matches := 0
	for _, term := range queryTerms {
		if keywordSet[term] {
			matches++
		}
	}

	return float64(matches)
}

// findCandidates returns documents that contain at least one query term.
func (idx *SearchIndex) findCandidates(queryTerms []string) map[int][]string {
	candidates := make(map[int][]string) // docID -> matched terms

	for _, term := range queryTerms {
		postings, exists := idx.Terms[term]
		if !exists {
			continue
		}

		for _, posting := range postings {
			candidates[posting.DocID] = append(candidates[posting.DocID], term)
		}
	}

	return candidates
}
