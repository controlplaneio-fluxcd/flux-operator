// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetFluxOperatorDocumentationResource creates a resource for providing
// Flux Operator API documentation in Markdown format.
func (m *Manager) GetFluxOperatorDocumentationResource() DocResource {
	return DocResource{
		Resource: mcp.NewResource(
			"doc://flux_operator_apis",
			"doc_flux_operator_apis",
			mcp.WithResourceDescription("The Flux Operator API documentation"),
			mcp.WithMIMEType("text/markdown"),
		),
		Handler: m.HandleGetFluxOperatorDocumentation,
	}
}

// HandleGetFluxOperatorDocumentation is the handler function for the doc_flux_operator_apis resource.
func (m *Manager) HandleGetFluxOperatorDocumentation(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	docs := []string{
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/fluxinstance.md",
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/resourceset.md",
		"https://raw.githubusercontent.com/controlplaneio-fluxcd/distribution/refs/heads/main/docs/operator/resourcesetinputprovider.md",
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
			URI:      "doc://flux_operator_apis",
			MIMEType: "text/markdown",
			Text:     stb.String(),
		},
	}, nil
}
