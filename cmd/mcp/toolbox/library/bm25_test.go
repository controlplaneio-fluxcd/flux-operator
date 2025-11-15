// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"math"
	"testing"
)

func TestBM25_IDF(t *testing.T) {
	// Create a simple test index
	index := &SearchIndex{
		Terms: map[string][]Posting{
			"common": {
				{DocID: 0, Frequency: 1},
				{DocID: 1, Frequency: 1},
				{DocID: 2, Frequency: 1},
			},
			"rare": {
				{DocID: 0, Frequency: 1},
			},
		},
		TotalDocs: 3,
	}

	// Test IDF calculation
	commonIDF := index.IDF("common")
	rareIDF := index.IDF("rare")

	// Rare terms should have higher IDF
	if rareIDF <= commonIDF {
		t.Errorf("IDF for rare term (%f) should be higher than common term (%f)", rareIDF, commonIDF)
	}

	// Non-existent term should have 0 IDF
	nonExistentIDF := index.IDF("nonexistent")
	if nonExistentIDF != 0.0 {
		t.Errorf("IDF for non-existent term should be 0.0, got %f", nonExistentIDF)
	}
}

func TestBM25_termFrequency(t *testing.T) {
	index := &SearchIndex{
		Terms: map[string][]Posting{
			"test": {
				{DocID: 0, Frequency: 3},
				{DocID: 1, Frequency: 1},
			},
		},
	}

	tests := []struct {
		term     string
		docID    int
		expected int
	}{
		{"test", 0, 3},
		{"test", 1, 1},
		{"test", 2, 0},
		{"nonexistent", 0, 0},
	}

	for _, tt := range tests {
		result := index.termFrequency(tt.term, tt.docID)
		if result != tt.expected {
			t.Errorf("termFrequency(%q, %d) = %d, want %d", tt.term, tt.docID, result, tt.expected)
		}
	}
}

func TestBM25_Score(t *testing.T) {
	// Create a test index with 5 documents (to ensure positive IDF)
	// "flux" appears in 2/5 docs, "helm" appears in 1/5 docs
	index := &SearchIndex{
		Terms: map[string][]Posting{
			"flux": {
				{DocID: 0, Frequency: 5},
				{DocID: 1, Frequency: 1},
			},
			"helm": {
				{DocID: 0, Frequency: 2},
			},
		},
		Documents: []SearchDocument{
			{ID: "doc0", Length: 100}, // has flux and helm
			{ID: "doc1", Length: 50},  // has flux only
			{ID: "doc2", Length: 75},  // empty
			{ID: "doc3", Length: 60},  // empty
			{ID: "doc4", Length: 90},  // empty
		},
		AvgDocLength: 75,
		TotalDocs:    5,
	}

	// Query for "flux helm"
	queryTerms := []string{"flux", "helm"}
	score0 := index.Score(queryTerms, 0)
	score1 := index.Score(queryTerms, 1)

	// Doc 0 should score higher (has both terms, more frequent)
	if score0 <= score1 {
		t.Errorf("Document 0 (score=%f) should score higher than Document 1 (score=%f)", score0, score1)
	}

	// Score should be positive when terms don't appear in all docs
	if score0 <= 0 {
		t.Errorf("BM25 score should be positive, got %f", score0)
	}

	// Non-matching query should give 0 score
	nonMatchingScore := index.Score([]string{"nonexistent"}, 0)
	if nonMatchingScore != 0 {
		t.Errorf("Non-matching query should give score 0, got %f", nonMatchingScore)
	}
}

func TestBM25_ScoreProperties(t *testing.T) {
	// Test that BM25 satisfies expected properties
	// Use 5 docs total so term appearing in 2/5 has positive IDF
	index := &SearchIndex{
		Terms: map[string][]Posting{
			"term": {
				{DocID: 0, Frequency: 1},
				{DocID: 1, Frequency: 10},
			},
		},
		Documents: []SearchDocument{
			{ID: "doc0", Length: 100},
			{ID: "doc1", Length: 100},
			{ID: "doc2", Length: 100},
			{ID: "doc3", Length: 100},
			{ID: "doc4", Length: 100},
		},
		AvgDocLength: 100,
		TotalDocs:    5,
	}

	score1 := index.Score([]string{"term"}, 0)
	score10 := index.Score([]string{"term"}, 1)

	// Higher frequency should give higher score
	if score10 <= score1 {
		t.Errorf("Document with TF=10 (score=%f) should score higher than TF=1 (score=%f)", score10, score1)
	}

	// But not 10x higher (saturation effect of BM25)
	if score10 >= score1*10 {
		t.Errorf("BM25 should saturate term frequency: score10=%f should be less than 10*score1=%f", score10, score1*10)
	}
}

func TestBM25_IDFFormula(t *testing.T) {
	// Test IDF calculation against expected formula
	// IDF = log((N - df + 0.5) / (df + 0.5))
	index := &SearchIndex{
		Terms: map[string][]Posting{
			"test": {{DocID: 0, Frequency: 1}},
		},
		TotalDocs: 10,
	}

	idf := index.IDF("test")
	N := 10.0
	df := 1.0
	expectedIDF := math.Log((N - df + 0.5) / (df + 0.5))

	if math.Abs(idf-expectedIDF) > 0.0001 {
		t.Errorf("IDF calculation incorrect: got %f, want %f", idf, expectedIDF)
	}
}
