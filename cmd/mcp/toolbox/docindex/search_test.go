// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	if err := Load(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestSearchEmbeddedArtifactRelevance(t *testing.T) {
	idx, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		query string
		trail string
	}{
		{query: "HelmRelease valuesFrom", trail: "HelmRelease > Writing a HelmRelease spec > Values"},
		{query: "scheduling deployment windows", trail: "Time-based delivery > Scheduling Configuration"},
		{query: "helmrelease values from configmap", trail: "HelmRelease > Writing a HelmRelease spec > Values"},
		{query: "ResourceSet dependsOn readyExpr", trail: "ResourceSet > Writing a ResourceSet spec > Dependency management"},
		// Matches the public docs MCP server, which ranks the Alert example first.
		{query: "Slack alerts", trail: "Alert > Example"},
		{query: "scheduling", trail: "Time-based delivery > Scheduling Configuration"},
		{query: "kustomizaton substituteFrom", trail: "Kustomization > Writing a Kustomization spec > Post build variable substitution"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			g := NewWithT(t)
			hits := idx.Search(tt.query, SearchOptions{Limit: 3})
			g.Expect(hits).ToNot(BeEmpty())
			g.Expect(hits[0].Chunk.HeadingTrail).To(Equal(tt.trail))
		})
	}
}

func TestSearchCamelCaseRoundTrip(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Fields"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", HeadingTrail: "Fields > Camel", Text: "Configure valuesFrom on the release."},
			{ID: 1, DocPath: "/docs/test", HeadingTrail: "Fields > Prose", Text: "Configure values from the release."},
		},
	)
	g.Expect(idx.Search("valuesFrom", SearchOptions{})).To(HaveLen(2))
	g.Expect(idx.Search("values from", SearchOptions{})).To(HaveLen(2))
}

func TestSearchCamelCaseWordScoresAsOneTerm(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", HeadingTrail: "Test > Plain", Text: "Kustomization cel"},
			{ID: 1, DocPath: "/docs/test", HeadingTrail: "Test > Camel", Text: strings.Repeat("HelmRelease ", 3)},
		},
	)
	// Two matched plain words must outrank one camelCase word, even though
	// the camelCase word expands to three index terms.
	hits := idx.Search("Kustomization HelmRelease cel", SearchOptions{})
	g.Expect(hits).To(HaveLen(2))
	g.Expect(hits[0].Chunk.ID).To(Equal(0))
}

func TestSearchEmbeddedArtifactCamelCaseKindBalance(t *testing.T) {
	g := NewWithT(t)
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())

	hits := idx.Search("Kustomization HelmRelease CEL", SearchOptions{Limit: 8})
	trails := make([]string, 0, len(hits))
	for _, hit := range hits {
		trails = append(trails, hit.Chunk.HeadingTrail)
	}
	g.Expect(trails).To(ContainElements(
		"Kustomization > Writing a Kustomization spec > Dependencies",
		"HelmRelease > Writing a HelmRelease spec > Dependencies",
		"HelmRelease > Writing a HelmRelease spec > Health check expressions",
	))
}

func TestSearchDocDescriptionIsIndexed(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{
			{Path: "/docs/described", Title: "Guide", Description: "Installing the operator on clusters"},
			{Path: "/docs/plain", Title: "Guide"},
		},
		[]Chunk{
			{ID: 0, DocPath: "/docs/plain", HeadingTrail: "Guide > Steps", Text: "Run the operator."},
			{ID: 1, DocPath: "/docs/described", HeadingTrail: "Guide > Steps", Text: "Run the operator."},
		},
	)
	hits := idx.Search("operator clusters", SearchOptions{})
	g.Expect(hits).To(HaveLen(2))
	g.Expect(hits[0].Chunk.ID).To(Equal(1))
	g.Expect(hits[0].Score).To(BeNumerically(">", hits[1].Score))
}

func TestSearchRepeatedQueryWordsCountOnce(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", HeadingTrail: "Test > One", Text: "alpha alpha"},
			{ID: 1, DocPath: "/docs/test", HeadingTrail: "Test > Two", Text: "beta gamma"},
		},
	)
	// Nothing matches all three words, so both hits come from the OR fallback:
	// chunk 1 matches two distinct words, chunk 0 matches one word twice.
	hits := idx.Search("alpha alpha beta gamma", SearchOptions{})
	g.Expect(hits).To(HaveLen(2))
	g.Expect([]int{hits[0].Chunk.ID, hits[1].Chunk.ID}).To(Equal([]int{1, 0}))
}

func TestSearchThreeCharacterTermsHaveNoFuzzyExpansion(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{{ID: 0, DocPath: "/docs/test", Text: "car"}},
	)
	g.Expect(idx.Search("cat", SearchOptions{})).To(BeEmpty())
}

func TestSearchExactPrefixAndFuzzyExpansions(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{{ID: 0, DocPath: "/docs/test", Text: "alerm alert alerts"}},
	)
	expansions := idx.inverted.expand("alert")
	g.Expect(expansions).To(HaveLen(3))
	g.Expect(expansions[0]).To(Equal(weightedTerm{term: "alert", weight: 1}))
	want := prefixWeight * 6 / (6 + 0.3)
	g.Expect(expansions[1].term).To(Equal("alerts"))
	g.Expect(expansions[1].weight).To(BeNumerically("~", want, 1e-12))
	g.Expect(expansions[2].term).To(Equal("alerm"))
	g.Expect(expansions[2].weight).To(BeNumerically("~", fuzzyWeight*5/6.0, 1e-12))
}

func TestSearchDeterministicOrderingForEqualScores(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 3, DocPath: "/docs/test", Text: "alpha"},
			{ID: 1, DocPath: "/docs/test", Text: "alpha"},
		},
	)
	hits := idx.Search("alpha", SearchOptions{})
	g.Expect(hits).To(HaveLen(2))
	g.Expect([]int{hits[0].Chunk.ID, hits[1].Chunk.ID}).To(Equal([]int{1, 3}))
}

func TestSearchThreeTermANDIntersection(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", Text: "alpha beta gamma"},
			{ID: 1, DocPath: "/docs/test", Text: "alpha beta"},
			{ID: 2, DocPath: "/docs/test", Text: "alpha gamma"},
			{ID: 3, DocPath: "/docs/test", Text: "beta gamma"},
		},
	)
	hits := idx.Search("alpha beta gamma", SearchOptions{})
	g.Expect(hits).To(HaveLen(4))
	g.Expect(hits[0].Chunk.ID).To(Equal(0))
}

func TestSearchANDHitsComeBeforeORHits(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{{Path: "/docs/test", Title: "Test"}},
		[]Chunk{
			{ID: 0, DocPath: "/docs/test", Text: "alpha beta"},
			{ID: 1, DocPath: "/docs/test", Text: strings.Repeat("alpha ", 20)},
		},
	)
	hits := idx.Search("alpha beta", SearchOptions{})
	g.Expect(hits).To(HaveLen(2))
	g.Expect(hits[0].Chunk.ID).To(Equal(0))
}

func TestSearchFilterAndLimit(t *testing.T) {
	g := NewWithT(t)
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	hits := idx.Search("verification", SearchOptions{
		Limit: 2,
		Filter: func(chunk *Chunk) bool {
			return strings.HasPrefix(chunk.DocPath, "/docs/crd/")
		},
	})
	g.Expect(hits).To(HaveLen(2))
	for _, hit := range hits {
		g.Expect(hit.Chunk.DocPath).To(HavePrefix("/docs/crd/"))
		g.Expect(hit.Chunk.DocPath).ToNot(HavePrefix("/docs/crds/"))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	g := NewWithT(t)
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(idx.Search("the and from", SearchOptions{})).To(BeEmpty())
}

func newTestIndex(docs []Doc, chunks []Chunk) *Index {
	idx := &Index{
		docs:         make([]*Doc, 0, len(docs)),
		chunks:       make([]*Chunk, 0, len(chunks)),
		docsByPath:   make(map[string]*Doc, len(docs)),
		chunksByPath: make(map[string][]*Chunk, len(docs)),
		chunksByID:   make(map[int]*Chunk, len(chunks)),
	}
	for i := range docs {
		doc := &docs[i]
		idx.docs = append(idx.docs, doc)
		idx.docsByPath[doc.Path] = doc
	}
	for i := range chunks {
		chunk := &chunks[i]
		idx.chunks = append(idx.chunks, chunk)
		idx.chunksByPath[chunk.DocPath] = append(idx.chunksByPath[chunk.DocPath], chunk)
		idx.chunksByID[chunk.ID] = chunk
	}
	idx.inverted = buildInvertedIndex(idx)
	return idx
}
