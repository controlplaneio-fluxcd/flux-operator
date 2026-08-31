// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"fmt"
	"strings"
)

const searchSnippetChars = 2000

// RenderHit renders a search hit in the MCP search tool's four-line shape.
func RenderHit(hit Hit) string {
	if hit.Chunk == nil || hit.Doc == nil {
		return ""
	}
	title := hit.Doc.Title
	trail := strings.Split(hit.Chunk.HeadingTrail, " > ")
	if len(trail) > 1 {
		title += " — " + strings.Join(trail[1:], " > ")
	}

	content := hit.Chunk.Text
	if runes := []rune(content); len(runes) > searchSnippetChars {
		cut := searchSnippetChars
		for i := searchSnippetChars; i > 0; i-- {
			if runes[i] == '\n' {
				cut = i
				break
			}
		}
		hint := fmt.Sprintf(`read_flux_doc path="%s" offset=%d`, hit.Doc.Path, hit.Chunk.StartLine)
		if hit.Chunk.Anchor != "" {
			hint = fmt.Sprintf(`read_flux_doc path="%s" heading="%s"`, hit.Doc.Path, hit.Chunk.Anchor)
		}
		content = string(runes[:cut]) + "\n… [truncated — " + hint + " for the rest]"
	}

	return fmt.Sprintf("Title: %s\nPath: %s\nLines: %d-%d of %d\nContent: %s",
		title, hit.Doc.Path, hit.Chunk.StartLine, hit.Chunk.EndLine, hit.Doc.LineCount, content)
}

// RenderNoResults renders the search response for a query with no hits.
func (idx *Index) RenderNoResults(query string) string {
	return fmt.Sprintf(`No results for "%s". Try different keywords, or restrict with path to one of: %s.`,
		query, strings.Join(idx.SectionPrefixes(), ", "))
}

// RenderUnknownPath renders a path miss with the closest documentation paths.
func (idx *Index) RenderUnknownPath(input string) string {
	return fmt.Sprintf(`Doc path "%s" was not found. Closest paths: %s.`,
		input, strings.Join(idx.ClosePathMatches(input), ", "))
}

const readMaxBytes = 30 * 1024

// ResolveHeading resolves heading by anchor first, then by case-insensitive heading text.
func ResolveHeading(doc *Doc, heading string) (h *Heading, note string, ok bool) {
	if doc == nil {
		return nil, "", false
	}

	needle := strings.TrimSpace(heading)
	anchor := strings.TrimPrefix(strings.ToLower(needle), "#")
	for i := range doc.Headings {
		if strings.EqualFold(doc.Headings[i].Anchor, anchor) {
			return &doc.Headings[i], "", true
		}
	}

	matches := make([]int, 0, 1)
	for i := range doc.Headings {
		if strings.EqualFold(doc.Headings[i].Text, needle) {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return nil, "", false
	}
	if len(matches) > 1 {
		others := make([]string, 0, len(matches)-1)
		for _, i := range matches[1:] {
			others = append(others, "#"+doc.Headings[i].Anchor)
		}
		note = fmt.Sprintf(`Note: "%s" matches %d headings; showing the first. Others: %s.`,
			heading, len(matches), strings.Join(others, ", "))
	}
	return &doc.Headings[matches[0]], note, true
}

// RenderOutline renders a document outline, optionally explaining that a heading was not found.
func RenderOutline(doc *Doc, heading string) string {
	if doc == nil {
		return ""
	}

	var b strings.Builder
	if heading == "" {
		fmt.Fprintf(&b, "Headings in %s (%s):", doc.Path, doc.Title)
	} else {
		fmt.Fprintf(&b, `Heading "%s" was not found in %s. Available headings (text — anchor):`, heading, doc.Path)
	}
	for _, h := range doc.Headings {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("  ", max(0, h.Level-1)))
		b.WriteString(h.Text)
		b.WriteString(" — ")
		b.WriteString(h.Anchor)
	}
	return b.String()
}

// RenderDoc renders a line-bounded document or heading section slice for the read tool.
func RenderDoc(doc *Doc, heading *Heading, offset, limit int, note string) string {
	if doc == nil {
		return ""
	}

	lines := strings.Split(doc.Body, "\n")
	total := len(lines)
	start := min(max(offset, 1), total)
	sectionEnd := total
	if heading != nil {
		start = heading.Line
		for _, next := range doc.Headings {
			if next.Line > heading.Line && next.Level <= heading.Level {
				sectionEnd = next.Line - 1
				break
			}
		}
	}

	maxEnd := min(start+limit-1, sectionEnd, total)
	out := make([]string, 0, maxEnd-start+1)
	bytes := 0
	end := start - 1
	for lineNumber := start; lineNumber <= maxEnd; lineNumber++ {
		line := lines[lineNumber-1]
		cost := len(line) + 1
		if bytes+cost > readMaxBytes && len(out) > 0 {
			break
		}
		out = append(out, line)
		bytes += cost
		end = lineNumber
	}

	nextHint := fmt.Sprintf(" Next: offset=%d.", end+1)
	if end == total {
		nextHint = " End of document."
	}
	header := fmt.Sprintf("Path: %s   Title: %s\nLines %d-%d of %d.%s", doc.Path, doc.Title, start, end, total, nextHint)
	if note != "" {
		header += "\n" + note
	}
	return header + "\n---\n" + strings.Join(out, "\n")
}
