// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

func TestManager_HandleSearchFluxDocs(t *testing.T) {
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "search_flux_docs",
		},
	}
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
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input searchFluxDocsInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, _, err := m.HandleSearchFluxDocs(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(result).ToNot(BeNil())
			g.Expect(result.Content).ToNot(BeEmpty())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			if test.matchErr != "" {
				g.Expect(result.IsError).To(BeTrue())
				g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			} else {
				g.Expect(result.IsError).To(BeFalse())
				for _, matchResult := range test.matchResults {
					g.Expect(textContent.Text).To(ContainSubstring(matchResult))
				}
			}
		})
	}
}
