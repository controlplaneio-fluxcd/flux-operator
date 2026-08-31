// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package docindex embeds the fluxoperator.dev documentation search index
// and implements the BM25 search, path resolution and rendering used by the
// MCP docs tools.
package docindex

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Artifact is the Flux documentation search artifact published by fluxoperator.dev.
type Artifact struct {
	SchemaVersion int             `json:"schemaVersion"`
	Version       string          `json:"version"`
	GeneratedAt   string          `json:"generatedAt"`
	Docs          []Doc           `json:"docs"`
	Chunks        []Chunk         `json:"chunks"`
	MiniSearch    json.RawMessage `json:"miniSearch,omitempty"`
}

// Doc describes a documentation page in an Artifact.
type Doc struct {
	Path         string    `json:"path"`
	Section      string    `json:"section"`
	SectionTitle string    `json:"sectionTitle"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	WordCount    int       `json:"wordCount"`
	LineCount    int       `json:"lineCount"`
	Headings     []Heading `json:"headings"`
	Body         string    `json:"-"`
}

// Heading describes a Markdown heading and its location in a Doc.
type Heading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
}

// Chunk describes a contiguous range of lines from a Doc.
type Chunk struct {
	ID           int    `json:"id"`
	DocPath      string `json:"docPath"`
	HeadingTrail string `json:"headingTrail"`
	Anchor       string `json:"anchor"`
	StartLine    int    `json:"startLine"`
	EndLine      int    `json:"endLine"`
	Text         string `json:"text"`
}

//go:embed index.json
var embeddedArtifact []byte

// Index provides access to the loaded documentation artifact.
type Index struct {
	artifact     *Artifact
	docs         []*Doc
	chunks       []*Chunk
	docsByPath   map[string]*Doc
	chunksByPath map[string][]*Chunk
	chunksByID   map[int]*Chunk
	inverted     *invertedIndex
}

var loadedIndex *Index

// Validate checks the artifact schema and reconstructs each Doc.Body from its chunks.
func Validate(artifact *Artifact) error {
	if artifact == nil {
		return errors.New("artifact is nil")
	}
	if artifact.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion %d, want 1", artifact.SchemaVersion)
	}
	if artifact.MiniSearch != nil {
		return errors.New("artifact contains miniSearch release index")
	}
	if len(artifact.Docs) == 0 {
		return errors.New("artifact contains no docs")
	}

	docsByPath := make(map[string]*Doc, len(artifact.Docs))
	headingAnchors := make(map[string]map[string]struct{}, len(artifact.Docs))
	for i := range artifact.Docs {
		doc := &artifact.Docs[i]
		if _, ok := docsByPath[doc.Path]; ok {
			return fmt.Errorf("duplicate doc path %q", doc.Path)
		}
		docsByPath[doc.Path] = doc

		anchors := make(map[string]struct{}, len(doc.Headings))
		for _, heading := range doc.Headings {
			if _, ok := anchors[heading.Anchor]; ok {
				return fmt.Errorf("doc %q has duplicate heading anchor %q at line %d", doc.Path, heading.Anchor, heading.Line)
			}
			anchors[heading.Anchor] = struct{}{}
		}
		headingAnchors[doc.Path] = anchors
	}

	chunksByPath := make(map[string][]*Chunk, len(artifact.Docs))
	for i := range artifact.Chunks {
		chunk := &artifact.Chunks[i]
		if chunk.ID != i {
			return fmt.Errorf("chunk at position %d has ID %d, want %d", i, chunk.ID, i)
		}
		if _, ok := docsByPath[chunk.DocPath]; !ok {
			return fmt.Errorf("chunk %d references missing doc path %q", chunk.ID, chunk.DocPath)
		}
		wantLines := chunk.EndLine - chunk.StartLine + 1
		gotLines := textLineCount(chunk.Text)
		if gotLines != wantLines {
			return fmt.Errorf("doc %q chunk %d at lines %d-%d has %d text lines, want %d",
				chunk.DocPath, chunk.ID, chunk.StartLine, chunk.EndLine, gotLines, wantLines)
		}
		chunksByPath[chunk.DocPath] = append(chunksByPath[chunk.DocPath], chunk)
	}

	for i := range artifact.Docs {
		doc := &artifact.Docs[i]
		chunks := chunksByPath[doc.Path]
		texts := make([]string, 0, len(chunks))
		nextLine := 1
		for _, chunk := range chunks {
			if chunk.StartLine != nextLine {
				return fmt.Errorf("doc %q chunk %d starts at line %d, want %d", doc.Path, chunk.ID, chunk.StartLine, nextLine)
			}
			if chunk.EndLine < chunk.StartLine {
				return fmt.Errorf("doc %q chunk %d ends at line %d before start line %d",
					doc.Path, chunk.ID, chunk.EndLine, chunk.StartLine)
			}
			texts = append(texts, chunk.Text)
			nextLine = chunk.EndLine + 1
		}
		if nextLine != doc.LineCount+1 {
			return fmt.Errorf("doc %q chunks end at line %d, want %d", doc.Path, nextLine-1, doc.LineCount)
		}

		doc.Body = strings.Join(texts, "\n")
		if gotLines := textLineCount(doc.Body); gotLines != doc.LineCount {
			return fmt.Errorf("doc %q reconstructed body has %d lines, want %d", doc.Path, gotLines, doc.LineCount)
		}
		bodyLines := strings.Split(doc.Body, "\n")
		for _, heading := range doc.Headings {
			if heading.Line < 1 || heading.Line > len(bodyLines) {
				return fmt.Errorf("doc %q heading %q has out-of-range line %d", doc.Path, heading.Text, heading.Line)
			}
			if !strings.HasPrefix(bodyLines[heading.Line-1], "#") {
				return fmt.Errorf("doc %q heading %q at line %d does not point to a Markdown heading",
					doc.Path, heading.Text, heading.Line)
			}
		}
		for _, chunk := range chunks {
			if chunk.Anchor == "" {
				continue
			}
			if _, ok := headingAnchors[doc.Path][chunk.Anchor]; !ok {
				return fmt.Errorf("doc %q chunk %d at lines %d-%d references missing heading anchor %q",
					doc.Path, chunk.ID, chunk.StartLine, chunk.EndLine, chunk.Anchor)
			}
		}
	}

	return nil
}

// Load parses and validates the embedded search artifact and makes it available through Get.
func Load() error {
	if loadedIndex != nil {
		return nil
	}

	var artifact Artifact
	if err := json.Unmarshal(embeddedArtifact, &artifact); err != nil {
		return fmt.Errorf("failed to parse embedded search index: %w", err)
	}
	if err := Validate(&artifact); err != nil {
		return fmt.Errorf("failed to validate embedded search index: %w", err)
	}

	idx := &Index{
		artifact:     &artifact,
		docs:         make([]*Doc, 0, len(artifact.Docs)),
		chunks:       make([]*Chunk, 0, len(artifact.Chunks)),
		docsByPath:   make(map[string]*Doc, len(artifact.Docs)),
		chunksByPath: make(map[string][]*Chunk, len(artifact.Docs)),
		chunksByID:   make(map[int]*Chunk, len(artifact.Chunks)),
	}
	for i := range artifact.Docs {
		doc := &artifact.Docs[i]
		idx.docs = append(idx.docs, doc)
		idx.docsByPath[doc.Path] = doc
	}
	for i := range artifact.Chunks {
		chunk := &artifact.Chunks[i]
		idx.chunks = append(idx.chunks, chunk)
		idx.chunksByPath[chunk.DocPath] = append(idx.chunksByPath[chunk.DocPath], chunk)
		idx.chunksByID[chunk.ID] = chunk
	}

	idx.inverted = buildInvertedIndex(idx)
	loadedIndex = idx
	return nil
}

// Get returns the loaded documentation index.
func Get() (*Index, error) {
	if loadedIndex == nil {
		return nil, errors.New("search index not available")
	}
	return loadedIndex, nil
}

// Docs returns the documentation pages in manifest order.
func (idx *Index) Docs() []*Doc {
	if idx == nil {
		return nil
	}
	return idx.docs
}

// DocByPath returns the documentation page whose Path exactly matches path.
func (idx *Index) DocByPath(path string) (*Doc, bool) {
	if idx == nil {
		return nil, false
	}
	doc, ok := idx.docsByPath[path]
	return doc, ok
}

// Chunks returns all documentation chunks in artifact order.
func (idx *Index) Chunks() []*Chunk {
	if idx == nil {
		return nil
	}
	return idx.chunks
}

// ChunksOf returns a document's chunks in artifact order.
func (idx *Index) ChunksOf(doc *Doc) []*Chunk {
	if idx == nil || doc == nil {
		return nil
	}
	return idx.chunksByPath[doc.Path]
}

// Version returns the source version recorded in the artifact.
func (idx *Index) Version() string {
	if idx == nil || idx.artifact == nil {
		return ""
	}
	return idx.artifact.Version
}

// GeneratedAt returns the artifact generation timestamp.
func (idx *Index) GeneratedAt() string {
	if idx == nil || idx.artifact == nil {
		return ""
	}
	return idx.artifact.GeneratedAt
}

// resetIndex clears the loaded index state for tests.
func resetIndex() {
	loadedIndex = nil
}

func textLineCount(text string) int {
	return strings.Count(text, "\n") + 1
}
