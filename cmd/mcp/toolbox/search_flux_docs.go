// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	if err := auth.CheckScopes(ctx, getScopeNames(ToolSearchFluxDocs, m.readOnly)); err != nil {
		return NewToolResultError(err.Error())
	}

	limit := int(input.Limit)
	if limit == 0 {
		limit = 1
	}

	library := NewLibrary()

	results := library.Search(input.Query, limit)
	if len(results) == 0 {
		return NewToolResultError("No documents found")
	}

	content, err := library.Fetch(results)
	if err != nil {
		return NewToolResultError(err.Error())
	}

	return NewToolResultText(content)
}
