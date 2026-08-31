// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestRenderHitTitles(t *testing.T) {
	g := NewWithT(t)
	doc := &Doc{Path: "/docs/test", Title: "Test", LineCount: 10}
	intro := RenderHit(Hit{Doc: doc, Chunk: &Chunk{HeadingTrail: "Test", StartLine: 1, EndLine: 2, Text: "intro"}})
	g.Expect(intro).To(HavePrefix("Title: Test\nPath: /docs/test\nLines: 1-2 of 10\nContent: intro"))
	nested := RenderHit(Hit{Doc: doc, Chunk: &Chunk{HeadingTrail: "Test > Parent > Child", StartLine: 3, EndLine: 4, Text: "body"}})
	g.Expect(nested).To(HavePrefix("Title: Test — Parent > Child\n"))
	g.Expect(nested).ToNot(ContainSubstring("Link:"))
}

func TestRenderHitTruncatesAtLineBoundaryWithHeading(t *testing.T) {
	g := NewWithT(t)
	lines := make([]string, 0, 60)
	for i := range 60 {
		lines = append(lines, fmt.Sprintf("line-%02d %s", i, strings.Repeat("x", 40)))
	}
	hit := Hit{
		Doc:   &Doc{Path: "/docs/test", Title: "Test", LineCount: 60},
		Chunk: &Chunk{HeadingTrail: "Test > Long", Anchor: "long", StartLine: 1, EndLine: 60, Text: strings.Join(lines, "\n")},
	}
	text := RenderHit(hit)
	g.Expect(text).To(ContainSubstring(`… [truncated — read_flux_doc path="/docs/test" heading="long" for the rest]`))
	content := strings.SplitN(text, "Content: ", 2)[1]
	g.Expect(len([]rune(strings.Split(content, "\n…")[0]))).To(BeNumerically("<=", searchSnippetChars))
	g.Expect(content).ToNot(ContainSubstring("line-59"))
}

func TestRenderHitTruncatesWithOffset(t *testing.T) {
	g := NewWithT(t)
	hit := Hit{
		Doc:   &Doc{Path: "/docs/test", Title: "Test", LineCount: 2},
		Chunk: &Chunk{HeadingTrail: "Test", StartLine: 42, EndLine: 43, Text: strings.Repeat("x", searchSnippetChars+1)},
	}
	g.Expect(RenderHit(hit)).To(ContainSubstring(`… [truncated — read_flux_doc path="/docs/test" offset=42 for the rest]`))
}

func TestRenderHitIncludesNewlineAtSnippetBoundary(t *testing.T) {
	g := NewWithT(t)
	content := strings.Repeat("a", 1000) + "\n" + strings.Repeat("b", 999) + "\ntrailing"
	hit := Hit{
		Doc:   &Doc{Path: "/docs/test", Title: "Test", LineCount: 3},
		Chunk: &Chunk{HeadingTrail: "Test", StartLine: 1, EndLine: 3, Text: content},
	}

	rendered := RenderHit(hit)
	snippet := strings.SplitN(strings.SplitN(rendered, "Content: ", 2)[1], "\n…", 2)[0]
	g.Expect([]rune(snippet)).To(HaveLen(searchSnippetChars))
	g.Expect(snippet).To(HaveSuffix(strings.Repeat("b", 999)))
}

func TestRenderMissTexts(t *testing.T) {
	g := NewWithT(t)
	small := newTestIndex([]Doc{{Path: "/docs/guides/install"}, {Path: "/docs/crd/helmrelease"}}, nil)
	g.Expect(small.RenderNoResults("nothing")).To(Equal(
		`No results for "nothing". Try different keywords, or restrict with path to one of: /docs/guides, /docs/crd.`,
	))
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(idx.RenderUnknownPath("/docs/crd/helmreleas")).To(HavePrefix(
		`Doc path "/docs/crd/helmreleas" was not found. Closest paths: /docs/crd/helmrelease`,
	))
}

func readTestDoc() *Doc {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	return &Doc{
		Path:      "/docs/crd/provider",
		Title:     "Provider",
		LineCount: len(lines),
		Body:      strings.Join(lines, "\n"),
		Headings: []Heading{
			{Level: 1, Text: "Providers", Anchor: "providers", Line: 1},
			{Level: 2, Text: "Example", Anchor: "example", Line: 5},
			{Level: 2, Text: "Writing a provider spec", Anchor: "writing-a-provider-spec", Line: 12},
			{Level: 3, Text: "Type", Anchor: "type", Line: 15},
			{Level: 4, Text: "Alerting", Anchor: "alerting", Line: 18},
			{Level: 5, Text: "Slack", Anchor: "slack", Line: 21},
			{Level: 5, Text: "Managed Identity", Anchor: "managed-identity", Line: 27},
			{Level: 5, Text: "Managed Identity", Anchor: "managed-identity-1", Line: 31},
			{Level: 2, Text: "Status", Anchor: "status", Line: 35},
		},
	}
}

func TestResolveHeading(t *testing.T) {
	g := NewWithT(t)
	doc := readTestDoc()

	h, note, ok := ResolveHeading(doc, "#MANAGED-IDENTITY-1")
	g.Expect(ok).To(BeTrue())
	g.Expect(h.Line).To(Equal(31))
	g.Expect(note).To(BeEmpty())

	h, note, ok = ResolveHeading(doc, "managed identity")
	g.Expect(ok).To(BeTrue())
	g.Expect(h.Line).To(Equal(27))
	g.Expect(note).To(Equal(`Note: "managed identity" matches 2 headings; showing the first. Others: #managed-identity-1.`))

	h, note, ok = ResolveHeading(doc, "sLaCk")
	g.Expect(ok).To(BeTrue())
	g.Expect(h.Line).To(Equal(21))
	g.Expect(note).To(BeEmpty())

	_, _, ok = ResolveHeading(doc, "nope")
	g.Expect(ok).To(BeFalse())
}

func TestRenderDocHeadingSection(t *testing.T) {
	g := NewWithT(t)
	doc := readTestDoc()

	text := RenderDoc(doc, &doc.Headings[5], 1, 400, "")
	g.Expect(text).To(ContainSubstring("Lines 21-26 of 40. Next: offset=27."))
	g.Expect(text).To(ContainSubstring("line 21"))
	g.Expect(text).To(ContainSubstring("line 26"))
	g.Expect(text).ToNot(ContainSubstring("line 27"))

	text = RenderDoc(doc, &doc.Headings[4], 1, 400, "")
	g.Expect(text).To(ContainSubstring("Lines 18-34 of 40. Next: offset=35."))
}

func TestRenderDocOffsetPaging(t *testing.T) {
	g := NewWithT(t)
	doc := readTestDoc()

	text := RenderDoc(doc, nil, 10, 5, "")
	g.Expect(text).To(ContainSubstring("Lines 10-14 of 40. Next: offset=15."))

	text = RenderDoc(doc, nil, 100, 5, "")
	g.Expect(text).To(ContainSubstring("Lines 40-40 of 40. End of document."))
	g.Expect(text).To(HaveSuffix("---\nline 40"))

	text = RenderDoc(doc, nil, 1, 400, "")
	g.Expect(text).To(ContainSubstring("Lines 1-40 of 40. End of document."))
}

func TestRenderDocThirtyKilobyteCap(t *testing.T) {
	g := NewWithT(t)
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("%s %d", strings.Repeat("x", 100), i+1)
	}
	doc := &Doc{Path: "/docs/big", Title: "Big", LineCount: len(lines), Body: strings.Join(lines, "\n")}
	text := RenderDoc(doc, nil, 1, 1000, "")
	var end, next int
	_, err := fmt.Sscanf(strings.Split(text, "\n")[1], "Lines 1-%d of 500. Next: offset=%d.", &end, &next)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(next).To(Equal(end + 1))
	g.Expect(end).To(BeNumerically("<", 500))
	payload := strings.SplitN(text, "---\n", 2)[1]
	g.Expect(len(payload)).To(BeNumerically("<=", readMaxBytes))

	longLine := strings.Repeat("y", readMaxBytes+100)
	doc = &Doc{Path: "/docs/one", Title: "One", LineCount: 2, Body: longLine + "\ntail"}
	text = RenderDoc(doc, nil, 1, 1000, "")
	g.Expect(text).To(ContainSubstring("Lines 1-1 of 2. Next: offset=2."))
	g.Expect(text).To(HaveSuffix("---\n" + longLine))
}

func TestRenderDocAmbiguousHeadingNote(t *testing.T) {
	g := NewWithT(t)
	doc := readTestDoc()
	h, note, ok := ResolveHeading(doc, "Managed Identity")
	g.Expect(ok).To(BeTrue())
	g.Expect(RenderDoc(doc, h, 1, 400, note)).To(ContainSubstring(note + "\n---"))
}

func TestRenderOutlineIndentation(t *testing.T) {
	g := NewWithT(t)
	doc := readTestDoc()
	outline := RenderOutline(doc, "")
	g.Expect(outline).To(HavePrefix("Headings in /docs/crd/provider (Provider):\nProviders — providers"))
	g.Expect(outline).To(ContainSubstring("\n        Slack — slack"))

	missing := RenderOutline(doc, "nope")
	g.Expect(missing).To(HavePrefix(`Heading "nope" was not found in /docs/crd/provider. Available headings (text — anchor):`))
	g.Expect(missing).To(ContainSubstring("\n  Example — example"))
}
