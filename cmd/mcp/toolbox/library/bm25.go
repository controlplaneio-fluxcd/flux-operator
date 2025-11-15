// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import "math"

const (
	// K1 is the term frequency saturation parameter for BM25
	K1 = 1.2
	// B is the length normalization parameter for BM25
	B = 0.75
)

// Score calculates the BM25 score for a document given query terms.
// BM25 formula:
// score = Σ IDF(qi) × (f(qi,D) × (k1+1)) / (f(qi,D) + k1 × (1-b + b × |D|/avgdl))
func (idx *SearchIndex) Score(queryTerms []string, docID int) float64 {
	score := 0.0
	doc := idx.Documents[docID]

	for _, term := range queryTerms {
		// Get term frequency in document
		tf := idx.termFrequency(term, docID)
		if tf == 0 {
			continue
		}

		// Calculate IDF
		idf := idx.IDF(term)

		// BM25 formula
		numerator := float64(tf) * (K1 + 1)
		denominator := float64(tf) + K1*(1-B+B*float64(doc.Length)/idx.AvgDocLength)
		score += idf * (numerator / denominator)
	}

	return score
}

// IDF calculates the inverse document frequency for a term.
// IDF formula with smoothing:
// IDF(qi) = log((N - df(qi) + 0.5) / (df(qi) + 0.5))
func (idx *SearchIndex) IDF(term string) float64 {
	postings, exists := idx.Terms[term]
	if !exists {
		return 0.0
	}

	df := float64(len(postings)) // document frequency
	N := float64(idx.TotalDocs)

	// IDF formula with smoothing
	return math.Log((N - df + 0.5) / (df + 0.5))
}

// termFrequency returns the frequency of a term in a specific document.
func (idx *SearchIndex) termFrequency(term string, docID int) int {
	postings, exists := idx.Terms[term]
	if !exists {
		return 0
	}

	for _, posting := range postings {
		if posting.DocID == docID {
			return posting.Frequency
		}
	}
	return 0
}
