// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true, "must": true,
	"can": true, "this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "as": true, "if": true, "when": true,
	"where": true, "why": true, "how": true, "all": true, "each": true,
	"every": true, "both": true, "few": true, "more": true, "most": true,
	"other": true, "some": true, "such": true, "no": true, "nor": true,
	"not": true, "only": true, "own": true, "same": true, "so": true,
	"than": true, "too": true, "very": true, "just": true, "now": true,
}

// Tokenize returns a normalized token stream while preserving repetitions.
// It follows MiniSearch by splitting on Unicode punctuation and separators and
// expanding camelCase into a whole token plus meaningful parts. Deliberately,
// and symmetrically for indexing and querying, it deviates only by removing stop
// words and dropping one-character non-version tokens.
func Tokenize(text string) []string {
	groups := TokenizeGroups(text)
	terms := make([]string, 0, len(groups))
	for _, group := range groups {
		terms = append(terms, group...)
	}
	return terms
}

// TokenizeGroups tokenizes text like Tokenize but keeps the variants of each
// input word together: a camelCase word yields one group holding the whole
// lowercased token followed by its parts, while a plain word yields a group of
// one. Words that produce no terms after filtering are omitted.
func TokenizeGroups(text string) [][]string {
	raw := splitTokens(text)

	groups := make([][]string, 0, len(raw))
	for _, token := range raw {
		variants := []string{strings.ToLower(token)}
		parts := camelCaseParts(token)
		if len(parts) > 1 {
			variants = append(variants, parts...)
		}
		group := make([]string, 0, len(variants))
		for _, variant := range variants {
			if variant == "" || stopWords[variant] {
				continue
			}
			if utf8.RuneCountInString(variant) == 1 && !isVersion(variant) {
				continue
			}
			group = append(group, variant)
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func splitTokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
}

func camelCaseParts(token string) []string {
	runes := []rune(token)
	parts := make([]string, 0, 4)
	start := 0
	for i := 1; i < len(runes); i++ {
		if (unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1])) && unicode.IsUpper(runes[i]) {
			if part := strings.ToLower(string(runes[start:i])); utf8.RuneCountInString(part) > 1 {
				parts = append(parts, part)
			}
			start = i
		}
	}
	if part := strings.ToLower(string(runes[start:])); utf8.RuneCountInString(part) > 1 {
		parts = append(parts, part)
	}
	return parts
}

func isVersion(s string) bool {
	runes := []rune(s)
	return len(runes) > 1 && runes[0] == 'v' && unicode.IsDigit(runes[1])
}
