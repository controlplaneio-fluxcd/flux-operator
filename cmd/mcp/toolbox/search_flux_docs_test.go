// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/toolbox/library"
)

func TestMain(m *testing.M) {
	if err := library.Load(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestManagerHandleSearchFluxDocs(t *testing.T) {
	request := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Name: ToolSearchFluxDocs}}
	manager := &Manager{}
	tests := []struct {
		name       string
		input      searchFluxDocsInput
		isError    bool
		contains   string
		items      int
		pathPrefix string
		exactPath  string
	}{
		{name: "one-character query rejected", input: searchFluxDocsInput{Query: "x"}, isError: true, contains: "query must be between"},
		{name: "200-character query accepted", input: searchFluxDocsInput{Query: strings.Repeat("z", 200)}, contains: "No results for", items: 1},
		{name: "fractional limit rejected", input: searchFluxDocsInput{Query: "dependsOn", Limit: 1.5}, isError: true, contains: "limit must be an integer"},
		{name: "negative limit rejected", input: searchFluxDocsInput{Query: "dependsOn", Limit: -1}, isError: true, contains: "limit must be an integer"},
		{name: "unknown path", input: searchFluxDocsInput{Query: "verification", Path: "/docs/crd/helmreleas"}, contains: "Doc path", items: 1},
		{name: "zero hits", input: searchFluxDocsInput{Query: "zzzzqxxxnotaword"}, contains: "No results for", items: 1},
		{name: "limit", input: searchFluxDocsInput{Query: "dependsOn", Limit: 2}, contains: "Title:", items: 2},
		{name: "doc path", input: searchFluxDocsInput{Query: "interval", Path: "/docs/crd/helmrelease", Limit: 3}, contains: "Path: /docs/crd/helmrelease", items: 3, exactPath: "/docs/crd/helmrelease"},
		{name: "section prefix", input: searchFluxDocsInput{Query: "verification", Path: "/docs/crd", Limit: 5}, contains: "Path: /docs/crd/", items: 5, pathPrefix: "/docs/crd/"},
		{name: "multiple content items", input: searchFluxDocsInput{Query: "HelmRelease valuesFrom", Limit: 3}, contains: "Content:", items: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result, _, err := manager.HandleSearchFluxDocs(context.Background(), request, tt.input)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result.IsError).To(Equal(tt.isError))
			if tt.items > 0 {
				g.Expect(result.Content).To(HaveLen(tt.items))
			}
			for _, item := range result.Content {
				text, ok := item.(*mcp.TextContent)
				g.Expect(ok).To(BeTrue())
				g.Expect(text.Text).To(ContainSubstring(tt.contains))
				if tt.pathPrefix != "" || tt.exactPath != "" {
					pathLine := ""
					for _, line := range strings.Split(text.Text, "\n") {
						if strings.HasPrefix(line, "Path: ") {
							pathLine = strings.TrimPrefix(line, "Path: ")
						}
					}
					if tt.pathPrefix != "" {
						g.Expect(pathLine).To(HavePrefix(tt.pathPrefix))
					}
					if tt.exactPath != "" {
						g.Expect(pathLine).To(Equal(tt.exactPath))
					}
				}
			}
		})
	}
}
