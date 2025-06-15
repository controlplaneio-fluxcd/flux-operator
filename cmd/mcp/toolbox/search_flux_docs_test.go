// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
)

func TestManager_HandleSearchFluxDocs(t *testing.T) {
	request := mcp.CallToolRequest{}
	request.Params.Name = "search_flux_docs"
	m := &Manager{}

	tests := []struct {
		testName     string
		arguments    map[string]any
		matchErr     string
		matchResults []string
	}{
		{
			testName: "fails with not found",
			arguments: map[string]any{
				"query": "notfound",
			},
			matchErr: "No documents found",
		},
		{
			testName: "returns single result",
			arguments: map[string]any{
				"query": "GitHub Pull Request",
			},
			matchResults: []string{"ResourceSetInputProvider"},
		},
		{
			testName: "returns multiple results",
			arguments: map[string]any{
				"query": "GitLab Merge Request",
				"limit": 2,
			},
			matchResults: []string{
				"ResourceSetInputProvider",
				"GitRepository",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			request.Params.Arguments = test.arguments

			result, err := m.HandleSearchFluxDocs(context.Background(), request)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := mcp.AsTextContent(result.Content[0])
			g.Expect(ok).To(BeTrue())

			if test.matchErr != "" {
				g.Expect(result.IsError).To(BeTrue())
				g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			} else {
				g.Expect(result.IsError).ToNot(BeTrue())
				for _, matchResult := range test.matchResults {
					g.Expect(textContent.Text).To(ContainSubstring(matchResult))
				}
			}
		})
	}
}
