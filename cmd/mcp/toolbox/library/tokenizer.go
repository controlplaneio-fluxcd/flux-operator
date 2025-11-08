// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	// stopWords contains common English words to filter out
	stopWords = map[string]bool{
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

	// Pattern to match word boundaries
	wordBoundary = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

	// Pattern to detect CamelCase
	camelCase = regexp.MustCompile(`([a-z])([A-Z])`)
)

// Tokenize converts text into normalized, searchable terms.
// It handles:
// - Lowercase normalization
// - Word boundary splitting (preserves hyphens in compound words)
// - CamelCase splitting (GitRepository -> git, repository)
// - Version preservation (v1, v2beta3)
// - Stop word removal
// - Flux/K8s-specific stemming
func Tokenize(text string) []string {
	// Split CamelCase words before lowercasing
	text = camelCase.ReplaceAllString(text, `${1} ${2}`)

	// Convert to lowercase
	text = strings.ToLower(text)

	// Split on word boundaries
	words := wordBoundary.Split(text, -1)

	// Track unique terms
	termsMap := make(map[string]bool)

	for _, word := range words {
		word = strings.TrimSpace(word)

		// Skip empty strings
		if word == "" {
			continue
		}

		// Skip very short words (< 2 chars) unless it looks like a version
		if len(word) < 2 && !isVersion(word) {
			continue
		}

		// Skip stop words
		if stopWords[word] {
			continue
		}

		// Apply stemming
		stemmed := stem(word)
		termsMap[stemmed] = true
	}

	// Convert map to slice
	terms := make([]string, 0, len(termsMap))
	for term := range termsMap {
		terms = append(terms, term)
	}

	return terms
}

// isVersion checks if a string looks like a version identifier (v1, v2beta3, etc.)
func isVersion(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] == 'v' && len(s) > 1 && unicode.IsDigit(rune(s[1]))
}

// stem applies simple stemming rules for common Flux/K8s terms
func stem(term string) string {
	// Remove common suffixes
	suffixes := []struct {
		suffix      string
		replacement string
	}{
		// Flux/K8s specific plurals
		{"repositories", "repository"},
		{"kustomizations", "kustomization"},
		{"helmreleases", "helmrelease"},
		{"helmrepositories", "helmrepository"},
		{"helmcharts", "helmchart"},
		{"gitrepositories", "gitrepository"},
		{"ocirepositories", "ocirepository"},
		{"buckets", "bucket"},
		{"receivers", "receiver"},
		{"alerts", "alert"},
		{"providers", "provider"},
		{"imagerepositories", "imagerepository"},
		{"imagepolicies", "imagepolicy"},
		{"imageupdateautomations", "imageupdateautomation"},
		{"artifactgenerators", "artifactgenerator"},

		// General plurals
		{"reconciliations", "reconciliation"},
		{"configurations", "configuration"},
		{"authentications", "authentication"},
		{"authorizations", "authorization"},
		{"specifications", "specification"},
		{"definitions", "definition"},
		{"deployments", "deployment"},
		{"namespaces", "namespace"},
		{"certificates", "certificate"},
		{"secrets", "secret"},
		{"configmaps", "configmap"},

		// Common suffixes (order matters - check longer first)
		{"ies", "y"},   // policies -> policy
		{"sses", "ss"}, // processes -> process
		{"ches", "ch"}, // patches -> patch
		{"shes", "sh"}, // pushes -> push
		{"xes", "x"},   // fixes -> fix
		{"zes", "z"},   // sizes -> size
		{"ses", "se"},  // releases -> release (fallback)
		{"s", ""},      // generic plural removal (last resort)
	}

	for _, suf := range suffixes {
		if strings.HasSuffix(term, suf.suffix) {
			if suf.replacement == "" {
				return term[:len(term)-len(suf.suffix)]
			}
			return strings.TrimSuffix(term, suf.suffix) + suf.replacement
		}
	}

	return term
}
