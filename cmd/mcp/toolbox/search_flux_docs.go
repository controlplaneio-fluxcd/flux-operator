// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// ToolSearchFluxDocs is the name of the search_flux_docs tool.
	ToolSearchFluxDocs = "search_flux_docs"
)

// NewSearchFluxDocsTool creates a new tool for searching Flux documentation.
func (m *Manager) NewSearchFluxDocsTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolSearchFluxDocs,
			mcp.WithDescription("This tool searches the Flux documentation and returns relevant up-to-date API specifications in markdown format."),
			mcp.WithString("query",
				mcp.Description("The search query."),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("The maximum number of matching documents to return. Default is 1."),
			),
		),
		Handler:   m.HandleSearchFluxDocs,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleSearchFluxDocs is the handler function for the search_flux_docs tool.
func (m *Manager) HandleSearchFluxDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolSearchFluxDocs, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	query := mcp.ParseString(request, "query", "")
	limit := mcp.ParseInt(request, "limit", 1)

	library := NewLibrary()

	results := library.Search(query, limit)
	if len(results) == 0 {
		return mcp.NewToolResultError("No documents found"), nil
	}

	content, err := library.Fetch(results)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(content), nil
}
