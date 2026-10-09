// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/docindex"
)

// docSection returns the line bounds of a heading section and the total line
// count of a doc from the embedded index, so that the read tool assertions
// do not depend on the exact content of the docs at the time of writing.
func docSection(t *testing.T, path, heading string) (start, end, total int) {
	t.Helper()
	g := NewWithT(t)
	idx, err := docindex.Get()
	g.Expect(err).ToNot(HaveOccurred())
	doc, ok := idx.ResolveDoc(path)
	g.Expect(ok).To(BeTrue())
	h, _, ok := docindex.ResolveHeading(doc, heading)
	g.Expect(ok).To(BeTrue())
	total = len(strings.Split(doc.Body, "\n"))
	start, end = h.Line, total
	for _, next := range doc.Headings {
		if next.Line > h.Line && next.Level <= h.Level {
			end = next.Line - 1
			break
		}
	}
	return start, end, total
}

func TestManagerHandleReadFluxDoc(t *testing.T) {
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: ToolReadFluxDoc}}
	manager := &Manager{}
	start, end, total := docSection(t, "/docs/crd/helmrelease", "values")
	singleLine := fmt.Sprintf("Lines %d-%d of %d.", start, start, total)
	tests := []struct {
		name     string
		input    readFluxDocInput
		isError  bool
		contains []string
	}{
		{
			name:     "heading read",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Heading: "values"},
			contains: []string{"Path: /docs/crd/helmrelease   Title: HelmRelease", fmt.Sprintf("Lines %d-%d of %d. Next: offset=%d.", start, end, total, end+1), "## Values"},
		},
		{
			name:     "offset and limit read",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Offset: float64(start), Limit: 20},
			contains: []string{fmt.Sprintf("Lines %d-%d of %d. Next: offset=%d.", start, start+19, total, start+20), "## Values"},
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
			contains: []string{"Path: /docs/crd/helmrelease", singleLine},
		},
		{
			name:     "hash heading returns outline",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease", Heading: "#"},
			contains: []string{`Heading "#" was not found in /docs/crd/helmrelease.`, "Available headings (text — anchor):"},
		},
		{
			name:     "md suffix",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease.md", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", singleLine},
		},
		{
			name:     "trailing slash",
			input:    readFluxDocInput{Path: "/docs/crd/helmrelease/", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", singleLine},
		},
		{
			name:     "full URL",
			input:    readFluxDocInput{Path: "https://fluxoperator.dev/docs/crd/helmrelease/", Heading: "values", Limit: 1},
			contains: []string{"Path: /docs/crd/helmrelease", singleLine},
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
