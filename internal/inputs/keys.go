// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"strings"
	"unicode"
)

// NormalizedKey is a type alias for string that represents
// a normalized key for use in templates.
type NormalizedKey string

// NormalizeKeyForTemplate normalizes the given string
// to a consistent format for use as a key in maps fed
// to Go templates.
//
// We convert uppercase letters to lowercase, replace
// spaces and punctuation with '_', and remove any
// characters not in [a-z0-9_]. Then we split the
// words by '_' and join only the resulting non-empty
// words back together with '_'.
func NormalizeKeyForTemplate(key string) NormalizedKey {
	// Map characters according to the rules above.
	key = strings.Map(func(r rune) rune {
		r = unicode.ToLower(r)
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return '_'
		}
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			return r
		}
		return -1
	}, key)

	// Split by '_' and rejoin non-empty words with '_'.
	var nonEmptyWords []string
	for word := range strings.SplitSeq(key, "_") {
		if word != "" {
			nonEmptyWords = append(nonEmptyWords, word)
		}
	}
	key = strings.Join(nonEmptyWords, "_")

	return NormalizedKey(key)
}
