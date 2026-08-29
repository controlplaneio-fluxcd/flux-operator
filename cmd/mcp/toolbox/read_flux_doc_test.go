// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

func TestManagerHandleReadFluxDoc(t *testing.T) {
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: ToolReadFluxDoc}}
	manager := &Manager{}
	tests := []struct {
		name     string
		input    readFluxDocInput
		isError  bool
		contains []string
	}{
		{
			name:     "heading read",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Heading: "values"},
			contains: []string{"Path: /docs/crd/helmrelease   Title: HelmRelease", "Lines 458-546 of 2339. Next: offset=547.", "## Values"},
		},
		{
			name:     "offset and limit read",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Offset: 458, Limit: 20},
			contains: []string{"Lines 458-477 of 2339. Next: offset=478.", "## Values"},
		},
		{
			name:     "unknown path",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelase"},
			contains: []string{`Doc path "/docs/crd/helmrelase" was not found. Closest paths: /docs/crd/helmrelease`},
		},
		{
			name:     "unknown heading",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Heading: "no-such-heading"},
			contains: []string{`Heading "no-such-heading" was not found in /docs/crd/helmrelease.`, "Available headings (text — anchor):", "Values — values"},
		},
		{
			name:     "invalid limit",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Limit: 1001},
			isError:  true,
			contains: []string{"limit must be an integer between 1 and 1000"},
		},
		{
			name:     "fractional offset rejected",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Offset: 1.5},
			isError:  true,
			contains: []string{"offset must be an integer greater than or equal to 1"},
		},
		{
			name:     "negative offset rejected",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Offset: -1},
			isError:  true,
			contains: []string{"offset must be an integer greater than or equal to 1"},
		},
		{
			name:     "fractional limit rejected",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Limit: 1.5},
			isError:  true,
			contains: []string{"limit must be an integer between 1 and 1000"},
		},
		{
			name:     "negative limit rejected",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Limit: -1},
			isError:  true,
			contains: []string{"limit must be an integer between 1 and 1000"},
		},
		{
			name:     "uppercase path",
			input:    readFluxDocInput{Path: "/DOCS/CRD/HELMRELEASE", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", "Lines 458-458 of 2339."},
		},
		{
			name:     "hash heading returns outline",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Heading: "#"},
			contains: []string{`Heading "#" was not found in /docs/crd/helmrelease.`, "Available headings (text — anchor):"},
		},
		{
			name:     "md suffix",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease.md", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", "Lines 458-458 of 2339."},
		},
		{
			name:     "trailing slash",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease/", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", "Lines 458-458 of 2339."},
		},
		{
			name:     "full URL",
			input:    readFluxDocInput{Path: "https://fluxoperator.dev/docs/crd/helmrelease/", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", "Lines 458-458 of 2339."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result, _, err := manager.HandleReadFluxDoc(context.Background(), request, tt.input)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result.IsError).To(Equal(tt.isError))
			g.Expect(result.Content).To(HaveLen(1))
			text, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())
			for _, expected := range tt.contains {
				g.Expect(text.Text).To(ContainSubstring(expected))
			}
		})
	}
}

func TestManagerHandleReadFluxDocAcceptsSearchScope(t *testing.T) {
	g := NewWithT(t)
	manager := &Manager{}
	ctx := WithScopes(context.Background(), []string{ScopesPrefix + ToolSearchFluxDocs})
	result, _, err := manager.HandleReadFluxDoc(ctx, &mcp.CallToolRequest{}, readFluxDocInput{
		Path: "/docs/crd/helmrelease", Heading: "values", Limit: 1,
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.IsError).To(BeFalse())
}
