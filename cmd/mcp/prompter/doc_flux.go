// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetFluxDocumentationResource creates a resource for providing
// Flux CD API documentation in Markdown format.
func (m *Manager) GetFluxDocumentationResource() DocResource {
	return DocResource{
		Resource: mcp.NewResource(
			"doc://flux_apis",
			"doc_flux_apis",
			mcp.WithResourceDescription("The Flux CD API documentation"),
			mcp.WithMIMEType("text/markdown"),
		),
		Handler: m.HandleGetFluxDocumentation,
	}
}

// HandleGetFluxDocumentation is the handler function for the doc_flux_apis resource.
func (m *Manager) HandleGetFluxDocumentation(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	docs := []string{
		"https://raw.githubusercontent.com/fluxcd/kustomize-controller/refs/heads/main/docs/spec/v1/kustomizations.md",
		"https://raw.githubusercontent.com/fluxcd/helm-controller/refs/heads/main/docs/spec/v2/helmreleases.md",
	}

	var stb strings.Builder
	for _, doc := range docs {
		markdown, err := m.fetchMarkdown(doc)
		if err != nil {
			return nil, fmt.Errorf("error fetching markdown: %v", err)
		}
		stb.WriteString(markdown)
		stb.WriteString("\n\n")
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "doc://flux_apis",
			MIMEType: "text/markdown",
			Text:     stb.String(),
		},
	}, nil
}
