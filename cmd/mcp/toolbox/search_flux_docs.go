// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/library"
)

const (
	// ToolSearchFluxDocs is the name of the search_flux_docs tool.
	ToolSearchFluxDocs = "search_flux_docs"
)

func init() {
	systemTools[ToolSearchFluxDocs] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// searchFluxDocsInput defines the input parameters for searching Flux documentation.
type searchFluxDocsInput struct {
	Query string  `json:"query" jsonschema:"The search query."`
	Limit float64 `json:"limit,omitempty" jsonschema:"The maximum number of matching documents to return. Default is 1."`
}

// HandleSearchFluxDocs is the handler function for the search_flux_docs tool.
func (m *Manager) HandleSearchFluxDocs(ctx context.Context, request *mcp.CallToolRequest, input searchFluxDocsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolSearchFluxDocs, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	limit := int(input.Limit)
	if limit == 0 {
		limit = 1
	}

	// Use the embedded search index
	index := library.GetSearchIndex()
	if index == nil {
		return NewToolResultError("Search index not available. Run 'make mcp-build-search-index' to build it.")
	}

	results := index.Search(input.Query, limit)
	if len(results) == 0 {
		return NewToolResultError("No documents found")
	}

	// Format results
	var content strings.Builder
	for i, result := range results {
		if i > 0 {
			content.WriteString("\n\n---\n\n")
		}

		// Add metadata header
		content.WriteString(fmt.Sprintf("# %s (%s)\n\n",
			result.Document.Metadata.Kind,
			result.Document.Metadata.Group))
		content.WriteString(fmt.Sprintf("**URL:** %s\n\n",
			result.Document.Metadata.URL))
		content.WriteString(fmt.Sprintf("**Score:** %v\n\n",
			result.Score))

		// Add document content
		content.WriteString(result.Document.Content)
	}

	return NewToolResultText(content.String())
}
