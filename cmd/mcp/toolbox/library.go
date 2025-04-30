// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// DocumentMetadata holds the metadata for a Flux document,
// including its URL, group, kind, and keywords.
type DocumentMetadata struct {
	URL      string   `json:"url"`
	Group    string   `json:"group"`
	Kind     string   `json:"kind"`
	Keywords []string `json:"keywords"`
}

// Library holds a collection of document references represented by the DocumentMetadata type.
// It provides methods to search in the metadata and fetch the document content.
type Library struct {
	Documents []DocumentMetadata `json:"documents"`
}

// NewLibrary initializes and returns a new Library
// instance populated with predefined DocumentMetadata data.
func NewLibrary() *Library {
	return &Library{
		Documents: docsMetadata,
	}
}

// Search queries the library's document metadata for matches based on the provided query string and optional limit.
// It ranks results by relevance score calculated from matches in kind, group, and keywords.
// Returns a sorted slice of FluxDocuments, limited to the specified maximum if a limit is provided.
func (l *Library) Search(query string, limit int) []DocumentMetadata {
	keywords := l.extractKeywords(query)

	// If no keywords are found, return documents based on the limit
	if len(keywords) == 0 {
		if limit > 0 && len(l.Documents) > limit {
			return l.Documents[:limit]
		}
		return l.Documents
	}

	type scoredDoc struct {
		doc   DocumentMetadata
		score int
	}

	var results []scoredDoc

	for _, doc := range l.Documents {
		score := 0

		for _, keyword := range keywords {
			// Check for exact matches in Kind (highest priority)
			if strings.Contains(strings.ToLower(doc.Kind), keyword) {
				score += 5
				// Bonus points for exact Kind match
				if strings.EqualFold(doc.Kind, keyword) {
					score += 10
				}
			}

			// Check for matches in the Group
			if strings.Contains(strings.ToLower(doc.Group), keyword) {
				score += 3
				// Bonus points for the exact Group match
				if strings.EqualFold(doc.Group, keyword) {
					score += 5
				}
			}

			// Check for matches in Keywords
			for _, docKeyword := range doc.Keywords {
				if strings.Contains(strings.ToLower(docKeyword), keyword) {
					score += 1
					// Bonus points for the exact keyword match
					if strings.EqualFold(docKeyword, keyword) {
						score += 2
					}
				}
			}
		}

		// Only include a document that matched something
		if score > 0 {
			results = append(results, scoredDoc{doc: doc, score: score})
		}
	}

	// Sort results by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Limit the number of results if specified
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	// Extract just the documents from the scored results
	matchedDocs := make([]DocumentMetadata, len(results))
	for i, sc := range results {
		matchedDocs[i] = sc.doc
	}

	return matchedDocs
}

// extractKeywords processes a query string to extract
// normalized, filtered, and unique keywords.
func (l *Library) extractKeywords(query string) []string {
	if query == "" {
		return []string{}
	}
	// Create a map to track extracted keywords
	keywordsMap := make(map[string]bool)

	// Normalize the remaining query text
	// Replace common separators with spaces
	normalized := strings.NewReplacer(
		"_", " ",
		".", " ",
		",", " ",
		";", " ",
		":", " ",
		"/", " ",
	).Replace(query)

	// Tokenize the normalized query
	tokens := strings.Fields(strings.ToLower(normalized))

	// Filter out common stop words and very short tokens
	stopWords := map[string]bool{
		"the": true, "and": true, "but": true, "for": true, "with": true,
		"from": true, "about": true, "like": true, "that": true, "this": true,
		"these": true, "those": true, "are": true, "was": true, "were": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"does": true, "did": true, "not": true, "just": true, "now": true,
		"then": true, "all": true, "any": true, "each": true, "every": true,
	}

	for _, token := range tokens {
		// Skip very short tokens and stop words
		if len(token) <= 2 || stopWords[token] {
			continue
		}

		keywordsMap[token] = true
	}

	// Convert map to slice
	keywords := make([]string, 0, len(keywordsMap))
	for keyword := range keywordsMap {
		keywords = append(keywords, keyword)
	}

	return keywords
}

// Fetch retrieves and concatenates the markdown content of the provided
// FluxDocuments by their URLs. Returns an error if any fail.
func (l *Library) Fetch(documents []DocumentMetadata) (string, error) {
	var stb strings.Builder
	for _, doc := range documents {
		markdown, err := l.fetchMarkdown(doc.URL)
		if err != nil {
			return "", fmt.Errorf("error fetching markdown: %v", err)
		}
		stb.WriteString(markdown)
		stb.WriteString("\n\n")
	}

	return stb.String(), nil
}

// fetchMarkdown retrieves the markdown content from the specified URL and returns it as a string.
// Returns an error if the request fails or the response is invalid.
func (l *Library) fetchMarkdown(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	return string(body), nil
}
