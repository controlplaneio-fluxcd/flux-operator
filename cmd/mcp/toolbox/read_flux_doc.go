// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/library"
)

const (
	// ToolReadFluxDoc is the name of the read_flux_doc tool.
	ToolReadFluxDoc = "read_flux_doc"
)

func init() {
	systemTools[ToolReadFluxDoc] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// readFluxDocInput defines the input parameters for reading Flux documentation.
type readFluxDocInput struct {
	Path    string  `json:"path" jsonschema:"Doc path from search results, e.g. /docs/crd/helmrelease"`
	Heading string  `json:"heading,omitempty" jsonschema:"Heading anchor or text; reads that section only"`
	Offset  float64 `json:"offset,omitempty" jsonschema:"1-based start line, ignored when heading is set"`
	Limit   float64 `json:"limit,omitempty" jsonschema:"Maximum lines to return (1-1000). Default is 400."`
}

// HandleReadFluxDoc is the handler function for the read_flux_doc tool.
func (m *Manager) HandleReadFluxDoc(ctx context.Context, request *mcp.CallToolRequest, input readFluxDocInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolReadFluxDoc, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	docs, err := library.Get()
	if err != nil {
		return NewToolResultError("search index not available. Run 'make mcp-build-search-index' to build it.")
	}

	pathLength := utf8.RuneCountInString(input.Path)
	if pathLength < 1 || pathLength > 300 {
		return NewToolResultError("path must be between 1 and 300 characters")
	}
	if utf8.RuneCountInString(input.Heading) > 300 {
		return NewToolResultError("heading must not exceed 300 characters")
	}
	offset := int(input.Offset)
	if input.Offset == 0 {
		offset = 1
	} else if offset < 1 || input.Offset != float64(offset) {
		return NewToolResultError("offset must be an integer greater than or equal to 1")
	}
	limit := int(input.Limit)
	if limit == 0 {
		limit = 400
	}
	if limit < 1 || limit > 1000 || (input.Limit != 0 && input.Limit != float64(limit)) {
		return NewToolResultError("limit must be an integer between 1 and 1000")
	}

	doc, found := docs.ResolveDoc(input.Path)
	if !found {
		return NewToolResultText(library.UnknownPathText(docs, input.Path))
	}
	if input.Heading != "" {
		heading, note, found := library.ResolveHeading(doc, input.Heading)
		if !found {
			return NewToolResultText(library.OutlineText(doc, input.Heading))
		}
		return NewToolResultText(library.SliceDoc(doc, heading, offset, limit, note))
	}
	return NewToolResultText(library.SliceDoc(doc, nil, offset, limit, ""))
}
