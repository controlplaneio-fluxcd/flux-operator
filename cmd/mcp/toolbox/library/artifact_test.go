// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestArtifactLoad(t *testing.T) {
	g := NewWithT(t)
	resetLibrary()

	_, err := Get()
	g.Expect(err).To(MatchError("search index not available"))

	g.Expect(Load()).To(Succeed())
	library, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(library).ToNot(BeNil())

	g.Expect(Load()).To(Succeed())
	again, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(again).To(BeIdenticalTo(library))
	g.Expect(library.Version()).To(Equal("main"))
	g.Expect(library.GeneratedAt()).ToNot(BeEmpty())

	docs := library.Docs()
	g.Expect(docs).ToNot(BeEmpty())
	for _, doc := range docs {
		g.Expect(textLineCount(doc.Body)).To(Equal(doc.LineCount), "doc %s body line count", doc.Path)

		anchors := make(map[string]struct{}, len(doc.Headings))
		for _, heading := range doc.Headings {
			anchors[heading.Anchor] = struct{}{}
		}

		nextLine := 1
		for _, chunk := range library.ChunksOf(doc) {
			g.Expect(chunk.StartLine).To(Equal(nextLine), "doc %s chunk %d order", doc.Path, chunk.ID)
			nextLine = chunk.EndLine + 1
			if chunk.Anchor != "" {
				_, ok := anchors[chunk.Anchor]
				g.Expect(ok).To(BeTrue(), "doc %s chunk %d anchor %s", doc.Path, chunk.ID, chunk.Anchor)
			}
		}
		g.Expect(nextLine).To(Equal(doc.LineCount+1), "doc %s chunks tile body", doc.Path)
	}
	g.Expect(library.Chunks()).ToNot(BeEmpty())

	doc, found := library.DocByPath("/docs/crd/helmrelease")
	g.Expect(found).To(BeTrue())
	g.Expect(doc).ToNot(BeNil())
	_, found = library.DocByPath("/docs/does-not-exist")
	g.Expect(found).To(BeFalse())
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Artifact)
		wantErr string
	}{
		{
			name: "bad schema version",
			mutate: func(artifact *Artifact) {
				artifact.SchemaVersion = 2
			},
			wantErr: "unsupported schemaVersion 2",
		},
		{
			name: "miniSearch present",
			mutate: func(artifact *Artifact) {
				artifact.MiniSearch = json.RawMessage(`{}`)
			},
			wantErr: "contains miniSearch",
		},
		{
			name: "zero docs",
			mutate: func(artifact *Artifact) {
				artifact.Docs = nil
				artifact.Chunks = nil
			},
			wantErr: "contains no docs",
		},
		{
			name: "duplicate doc path",
			mutate: func(artifact *Artifact) {
				artifact.Docs = append(artifact.Docs, artifact.Docs[0])
			},
			wantErr: `duplicate doc path "/docs/test"`,
		},
		{
			name: "chunk references missing doc",
			mutate: func(artifact *Artifact) {
				artifact.Chunks[0].DocPath = "/docs/missing"
			},
			wantErr: `chunk 0 references missing doc path "/docs/missing"`,
		},
		{
			name: "tiling gap",
			mutate: func(artifact *Artifact) {
				artifact.Chunks[1].StartLine = 4
				artifact.Chunks[1].EndLine = 4
			},
			wantErr: `doc "/docs/test" chunk 1 starts at line 4, want 3`,
		},
		{
			name: "wrong chunk text line count",
			mutate: func(artifact *Artifact) {
				artifact.Chunks[0].Text = "# Test"
			},
			wantErr: `doc "/docs/test" chunk 0 at lines 1-2 has 1 text lines, want 2`,
		},
		{
			name: "duplicate heading anchor",
			mutate: func(artifact *Artifact) {
				artifact.Docs[0].Headings[1].Anchor = "test"
			},
			wantErr: `doc "/docs/test" has duplicate heading anchor "test" at line 3`,
		},
		{
			name: "chunk anchor missing from headings",
			mutate: func(artifact *Artifact) {
				artifact.Chunks[1].Anchor = "missing"
			},
			wantErr: `doc "/docs/test" chunk 1 at lines 3-3 references missing heading anchor "missing"`,
		},
		{
			name: "heading line is not a heading",
			mutate: func(artifact *Artifact) {
				artifact.Docs[0].Headings[1].Line = 2
			},
			wantErr: `doc "/docs/test" heading "Details" at line 2 does not point to a Markdown heading`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			artifact := validTestArtifact()
			tt.mutate(artifact)
			err := Validate(artifact)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
		})
	}
}

func TestValidateParsesMiniSearchPresence(t *testing.T) {
	g := NewWithT(t)
	var artifact Artifact
	g.Expect(json.Unmarshal([]byte(`{"schemaVersion":1,"miniSearch":null,"docs":[]}`), &artifact)).To(Succeed())
	g.Expect(artifact.MiniSearch).ToNot(BeNil())
	g.Expect(Validate(&artifact)).To(MatchError(ContainSubstring("contains miniSearch")))
}

func validTestArtifact() *Artifact {
	return &Artifact{
		SchemaVersion: 1,
		Version:       "main",
		GeneratedAt:   "2026-08-29T00:00:00Z",
		Docs: []Doc{
			{
				Path:      "/docs/test",
				Title:     "Test",
				LineCount: 3,
				Headings: []Heading{
					{Level: 1, Text: "Test", Anchor: "test", Line: 1},
					{Level: 2, Text: "Details", Anchor: "details", Line: 3},
				},
			},
		},
		Chunks: []Chunk{
			{ID: 0, DocPath: "/docs/test", Anchor: "test", StartLine: 1, EndLine: 2, Text: strings.Join([]string{"# Test", "body"}, "\n")},
			{ID: 1, DocPath: "/docs/test", Anchor: "details", StartLine: 3, EndLine: 3, Text: "## Details"},
		},
	}
}
