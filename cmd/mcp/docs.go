// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	mcpgolang "github.com/metoro-io/mcp-golang"
)

type Documentation struct {
	Path        string
	Name        string
	Description string
	ContentType string
	Handler     any
}

var DocumentationList = []Documentation{
	{
		Path:        "https://fluxcd.io/flux/components/",
		Name:        "flux_api_docs",
		Description: "Flux CD API documentation",
		ContentType: "text/markdown",
		Handler:     GetFluxDocsHandler,
	},
	{
		Path:        "https://fluxcd.control-plane.io/operator/",
		Name:        "flux_operator_api_docs",
		Description: "Flux Operator API documentation",
		ContentType: "text/markdown",
		Handler:     GetFluxOperatorDocsHandler,
	},
}

func GetFluxOperatorDocsHandler(ctx context.Context) (*mcpgolang.ResourceResponse, error) {
	docs := []string{
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/fluxinstance.md",
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/resourceset.md",
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/resourcesetinputprovider.md",
	}

	var stb strings.Builder
	for _, doc := range docs {
		markdown, err := fetchMarkdown(doc)
		if err != nil {
			return nil, fmt.Errorf("error fetching markdown: %v", err)
		}
		stb.WriteString(markdown)
		stb.WriteString("\n\n")
	}

	return mcpgolang.NewResourceResponse(
		mcpgolang.NewTextEmbeddedResource(
			"https://fluxcd.control-plane.io/operator/",
			stb.String(),
			"text/markdown"),
	), nil
}

func GetFluxDocsHandler(ctx context.Context) (*mcpgolang.ResourceResponse, error) {
	docs := []string{
		"https://raw.githubusercontent.com/fluxcd/kustomize-controller/refs/heads/main/docs/spec/v1/kustomizations.md",
		"https://raw.githubusercontent.com/fluxcd/helm-controller/refs/heads/main/docs/spec/v2/helmreleases.md",
	}

	var stb strings.Builder
	for _, doc := range docs {
		markdown, err := fetchMarkdown(doc)
		if err != nil {
			return nil, fmt.Errorf("error fetching markdown: %v", err)
		}
		stb.WriteString(markdown)
		stb.WriteString("\n\n")
	}

	return mcpgolang.NewResourceResponse(
		mcpgolang.NewTextEmbeddedResource(
			"https://fluxcd.io/flux/components/",
			stb.String(),
			"text/markdown"),
	), nil
}

func fetchMarkdown(url string) (string, error) {
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
