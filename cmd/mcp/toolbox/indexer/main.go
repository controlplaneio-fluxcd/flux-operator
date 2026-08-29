// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/library"
)

const (
	defaultIndexURL = "https://fluxoperator.dev/mcp/docs-index-main.json"
	outputPath      = "cmd/mcp/toolbox/library/index.json"
	maxIndexSize    = 10 * 1024 * 1024
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error building MCP docs index: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	source := os.Getenv("FLUX_DOCS_INDEX_URL")
	if source == "" {
		source = defaultIndexURL
	}

	data, err := readIndex(source)
	if err != nil {
		return err
	}

	var artifact library.Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return fmt.Errorf("failed to parse docs index: %w", err)
	}
	if err := library.Validate(&artifact); err != nil {
		return fmt.Errorf("failed to validate docs index: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write docs index to %s: %w", outputPath, err)
	}

	fmt.Printf("docs=%d chunks=%d bytes=%d version=%q generatedAt=%q\n",
		len(artifact.Docs), len(artifact.Chunks), len(data), artifact.Version, artifact.GeneratedAt)
	return nil
}

func readIndex(source string) ([]byte, error) {
	if !strings.HasPrefix(source, "http") {
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed to read docs index from %s: %w", source, err)
		}
		if len(data) > maxIndexSize {
			return nil, fmt.Errorf("docs index from %s exceeds 10 MB", source)
		}
		return data, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("failed to download docs index from %s: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download docs index from %s: unexpected status %s", source, resp.Status)
	}
	if resp.ContentLength > maxIndexSize {
		return nil, fmt.Errorf("docs index from %s exceeds 10 MB", source)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIndexSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read docs index from %s: %w", source, err)
	}
	if len(data) > maxIndexSize {
		return nil, fmt.Errorf("docs index from %s exceeds 10 MB", source)
	}
	return data, nil
}
