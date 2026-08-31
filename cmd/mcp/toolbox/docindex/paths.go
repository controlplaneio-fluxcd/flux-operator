// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"sort"
	"strings"
)

const canonicalOrigin = "https://fluxoperator.dev"

// NormalizePath normalizes a documentation path or fluxoperator.dev URL.
func NormalizePath(input string) string {
	path := strings.ToLower(strings.TrimSpace(input))
	path = strings.TrimPrefix(path, canonicalOrigin)
	path = "/" + strings.TrimLeft(path, "/")
	path = strings.TrimRight(path, "/")
	path = strings.TrimSuffix(path, ".md")
	if path == "" {
		return "/"
	}
	return path
}

// ResolveDoc resolves input to an exact documentation page.
func (idx *Index) ResolveDoc(input string) (*Doc, bool) {
	if idx == nil {
		return nil, false
	}
	doc, found := idx.docsByPath[NormalizePath(input)]
	return doc, found
}

// IsSectionPrefix reports whether input is a segment-aware documentation section prefix.
func (idx *Index) IsSectionPrefix(input string) bool {
	if idx == nil {
		return false
	}
	prefix := NormalizePath(input) + "/"
	for _, doc := range idx.docs {
		if strings.HasPrefix(doc.Path, prefix) {
			return true
		}
	}
	return false
}

// ClosePathMatches returns up to five likely documentation paths for input.
func (idx *Index) ClosePathMatches(input string) []string {
	if idx == nil {
		return nil
	}
	needle := []rune(NormalizePath(input))
	if len(needle) > 300 {
		needle = needle[:300]
	}
	needleText := string(needle)
	substring := strings.TrimPrefix(strings.TrimPrefix(needleText, "/docs/"), "/")
	type candidate struct {
		path     string
		distance int
		includes bool
	}
	candidates := make([]candidate, 0, len(idx.docs))
	for _, doc := range idx.docs {
		candidates = append(candidates, candidate{
			path:     doc.Path,
			distance: levenshteinDistance(needleText, doc.Path),
			includes: strings.Contains(doc.Path, substring),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].includes != candidates[j].includes {
			return candidates[i].includes
		}
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].path < candidates[j].path
	})
	limit := min(5, len(candidates))
	matches := make([]string, limit)
	for i := range limit {
		matches[i] = candidates[i].path
	}
	return matches
}

// SectionPrefixes returns distinct /docs/<section> prefixes in manifest order.
func (idx *Index) SectionPrefixes() []string {
	if idx == nil {
		return nil
	}
	prefixes := make([]string, 0, 8)
	seen := make(map[string]struct{})
	for _, doc := range idx.docs {
		parts := strings.Split(strings.Trim(doc.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "docs" {
			continue
		}
		prefix := "/docs/" + parts[1]
		if _, found := seen[prefix]; found {
			continue
		}
		seen[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func levenshteinDistance(left, right string) int {
	a, b := []rune(left), []rune(right)
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}
