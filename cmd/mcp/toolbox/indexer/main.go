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
	const outputPath = "cmd/mcp/toolbox/library/index.db"

	corpora := []struct {
		format   library.IndexFormat
		metadata []library.DocumentMetadata
	}{
		{
			format:   library.IndexFormatConcise,
			metadata: library.GetConciseDocsMetadata(),
		},
		{
			format:   library.IndexFormatComplete,
			metadata: library.GetCompleteDocsMetadata(),
		},
	}

	db := &library.SearchDatabase{
		Indexes: make(map[library.IndexFormat]*library.SearchIndex, len(corpora)),
	}

	for _, corpus := range corpora {
		fmt.Printf("Building %s search index for Flux documentation...\n", corpus.format)
		fmt.Printf("Found %d documents to index\n", len(corpus.metadata))

		index, err := buildIndex(corpus.metadata)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building %s search index: %v\n", corpus.format, err)
			os.Exit(1)
		}

		fmt.Printf("Successfully built %s search index with %d documents, %d unique terms\n",
			corpus.format, index.TotalDocs, len(index.Terms))
		fmt.Printf("Average document length: %.0f words\n\n", index.AvgDocLength)

		db.Indexes[corpus.format] = index
	}

	if err := saveDatabase(db, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving search database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Search database saved to %s\n", outputPath)
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

	for _, meta := range metadata {
		fmt.Printf("[%d/%d] Downloading %s...\n", len(index.Documents)+1, len(metadata), meta.Label())

		// Download document
		content, err := downloadMarkdown(client, meta.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to download %s: %w", meta.URL, err)
		}

		// Create document
		doc := library.SearchDocument{
			ID:       meta.ID(),
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
				DocID:     len(index.Documents),
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

// saveDatabase serializes the search database to a file using gob encoding
func saveDatabase(db *library.SearchDatabase, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(db); err != nil {
		return fmt.Errorf("failed to encode database: %w", err)
	}

	return nil
}
