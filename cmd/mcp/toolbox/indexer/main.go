// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/library"
)

func main() {
	fmt.Println("Building search index for Flux documentation...")

	// Load document metadata from library_index.go
	docs := library.GetDocsMetadata()
	fmt.Printf("Found %d documents to index\n", len(docs))

	// Build the search index
	index, err := buildIndex(docs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building index: %v\n", err)
		os.Exit(1)
	}

	// Save the index to library/index.gob
	outputPath := "cmd/mcp/toolbox/library/index.gob"
	if err := saveIndex(index, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving index: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully built search index with %d documents, %d unique terms\n",
		index.TotalDocs, len(index.Terms))
	fmt.Printf("Average document length: %.0f words\n", index.AvgDocLength)
	fmt.Printf("Index saved to %s\n", outputPath)
}

// buildIndex downloads documents and creates the search index
func buildIndex(metadata []library.DocumentMetadata) (*library.SearchIndex, error) {
	index := &library.SearchIndex{
		Terms:     make(map[string][]library.Posting),
		Documents: make([]library.SearchDocument, 0, len(metadata)),
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for i, meta := range metadata {
		fmt.Printf("[%d/%d] Downloading %s/%s...\n", i+1, len(metadata), meta.Group, meta.Kind)

		// Download document
		content, err := downloadMarkdown(client, meta.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", meta.URL, err)
		}

		// Create document
		doc := library.SearchDocument{
			ID:       fmt.Sprintf("%s-%s", meta.Group, meta.Kind),
			Content:  content,
			Metadata: meta,
		}

		// Tokenize
		tokens := library.Tokenize(content)
		doc.Length = len(tokens)

		// Build term frequency map
		termFreq := make(map[string]int)
		for _, term := range tokens {
			termFreq[term]++
		}

		// Add to inverted index
		for term, freq := range termFreq {
			index.Terms[term] = append(index.Terms[term], library.Posting{
				DocID:     i,
				Frequency: freq,
			})
		}

		index.Documents = append(index.Documents, doc)
		fmt.Printf("  Indexed %d words, %d unique terms\n", doc.Length, len(termFreq))
	}

	// Calculate average document length
	totalLength := 0
	for _, doc := range index.Documents {
		totalLength += doc.Length
	}
	index.AvgDocLength = float64(totalLength) / float64(len(index.Documents))
	index.TotalDocs = len(index.Documents)

	return index, nil
}

// downloadMarkdown fetches markdown content from a URL
func downloadMarkdown(client *http.Client, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	return string(body), nil
}

// saveIndex serializes the index to a file using gob encoding
func saveIndex(idx *library.SearchIndex, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(idx); err != nil {
		return fmt.Errorf("failed to encode index: %w", err)
	}

	return nil
}
