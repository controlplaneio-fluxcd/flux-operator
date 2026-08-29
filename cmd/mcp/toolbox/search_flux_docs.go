// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"strings"
	"unicode/utf8"

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
	Query string  `json:"query" jsonschema:"Keywords naming the kind and field, e.g. HelmRelease valuesFrom (2-200 characters)."`
	Path  string  `json:"path,omitempty" jsonschema:"Optional doc or section path (max 300 characters): /docs/guides, /docs/instance, /docs/resourcesets, /docs/web-ui, /docs/mcp, /docs/crd, /docs/controllers, /docs/charts."`
	Limit float64 `json:"limit,omitempty" jsonschema:"Maximum sections to return (1-20). Default is 8."`
}

// HandleSearchFluxDocs is the handler function for the search_flux_docs tool.
func (m *Manager) HandleSearchFluxDocs(ctx context.Context, request *mcp.CallToolRequest, input searchFluxDocsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolSearchFluxDocs, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	docs, err := library.Get()
	if err != nil {
		return NewToolResultError("search index not available. Run 'make mcp-build-search-index' to build it.")
	}

	queryLength := utf8.RuneCountInString(input.Query)
	if queryLength < 2 || queryLength > 200 {
		return NewToolResultError("query must be between 2 and 200 characters")
	}
	if utf8.RuneCountInString(input.Path) > 300 {
		return NewToolResultError("path must not exceed 300 characters")
	}
	limit := int(input.Limit)
	if limit == 0 {
		limit = 8
	}
	if limit < 1 || limit > 20 || (input.Limit != 0 && input.Limit != float64(limit)) {
		return NewToolResultError("limit must be an integer between 1 and 20")
	}

	options := library.SearchOptions{Limit: limit}
	if input.Path != "" {
		if doc, found := docs.ResolveDoc(input.Path); found {
			options.Filter = func(chunk *library.Chunk) bool {
				return chunk.DocPath == doc.Path
			}
		} else if docs.IsSectionPrefix(input.Path) {
			prefix := library.NormalizePath(input.Path) + "/"
			options.Filter = func(chunk *library.Chunk) bool {
				return strings.HasPrefix(chunk.DocPath, prefix)
			}
		} else {
			return NewToolResultText(library.UnknownPathText(docs, input.Path))
		}
	}

	hits := docs.Search(input.Query, options)
	if len(hits) == 0 {
		return NewToolResultText(library.NoResultsText(input.Query, docs.SectionPrefixes()))
	}
	texts := make([]string, len(hits))
	for i, hit := range hits {
		texts[i] = library.RenderSearchHit(hit)
	}
	return NewToolResultTexts(texts)
}
