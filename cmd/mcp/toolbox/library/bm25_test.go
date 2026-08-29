// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"math"
	"testing"

	. "github.com/onsi/gomega"
)

func TestCalcBM25Score(t *testing.T) {
	g := NewWithT(t)
	g.Expect(calcBM25Score(2, 1, 10, 4, 5)).To(BeNumerically("~", 3.887604503399492, 1e-12))
	g.Expect(calcBM25Score(1, 3, 3, 2, 2)).To(BeNumerically("~", 0.20029708893678386, 1e-12))
	g.Expect(calcBM25Score(1, 3, 3, 2, 2)).To(BeNumerically(">", 0), "IDF must stay non-negative when every chunk matches")
}

func TestCalcBM25ScoreIncludesDFloor(t *testing.T) {
	g := NewWithT(t)
	idf := math.Log(1 + (2.0-1+0.5)/(1+0.5))
	withoutFloor := idf * (1 * (bm25K + 1) / (1 + bm25K))
	g.Expect(calcBM25Score(1, 1, 2, 1, 1) - withoutFloor).To(BeNumerically("~", idf*bm25D, 1e-12))
}

func TestIndexUsesUniqueRawTokenFieldLength(t *testing.T) {
	g := NewWithT(t)
	library := newTestLibrary(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", Text: "valuesFrom"},
			{ID: 1, DocPath: "/docs/test", Text: "values from"},
		},
	)
	textField := library.index.fields[2]
	g.Expect(textField.fieldLengths[0]).To(Equal(1))
	g.Expect(textField.fieldLengths[1]).To(Equal(2))
	g.Expect(textField.averageFieldLength).To(Equal(1.5))
	g.Expect(textField.postings["valuesfrom"][0].frequency).To(Equal(1))
	g.Expect(textField.postings["values"]).To(HaveLen(2))
}

func TestQualityMultiplierRewardsDistinctTerms(t *testing.T) {
	g := NewWithT(t)
	library := newTestLibrary(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", Text: "alpha beta"},
			{ID: 1, DocPath: "/docs/test", Text: "alpha"},
		},
	)
	results := map[int]*scoredChunk{
		0: {score: 2, matched: map[string]struct{}{"alpha": {}, "beta": {}}},
		1: {score: 3, matched: map[string]struct{}{"alpha": {}}},
	}
	hits := library.rank(results)
	g.Expect(hits[0].Chunk.ID).To(Equal(0))
	g.Expect(hits[0].Score).To(Equal(4.0))
	g.Expect(hits[1].Score).To(Equal(3.0))
}
