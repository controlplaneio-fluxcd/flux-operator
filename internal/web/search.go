// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import "strings"

// hasWildcard returns true if the pattern contains wildcard characters.
func hasWildcard(pattern string) bool {
	return pattern != "" && strings.Contains(pattern, "*")
}

// matchesWildcard checks if a name matches a pattern with wildcard support.
// Supports * (matches any characters). If no wildcards present, does exact match.
// Matching is case-insensitive.
func matchesWildcard(name, pattern string) bool {
	name = strings.ToLower(name)
	pattern = strings.ToLower(pattern)

	// If no wildcards, do exact match
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}

	// Split pattern by * and check each segment appears in order
	segments := strings.Split(pattern, "*")
	pos := 0

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		idx := strings.Index(name[pos:], segment)
		if idx == -1 {
			return false
		}

		// First segment must be at the start (unless pattern starts with *)
		if i == 0 && idx != 0 {
			return false
		}

		pos += idx + len(segment)
	}

	// Last segment must be at the end (unless pattern ends with *)
	if len(segments) > 0 && segments[len(segments)-1] != "" && pos != len(name) {
		return false
	}

	return true
}
