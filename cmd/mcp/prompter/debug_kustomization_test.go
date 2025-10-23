// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

func TestManager_HandleDebugKustomization(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		testName     string
		arguments    map[string]string
		matchErr     string
		matchMessage string
	}{
		{
			testName: "fails without name",
			arguments: map[string]string{
				"kind": "HelmRelease",
			},
			matchErr: "missing name argument",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]string{
				"name": "test",
			},
			matchErr: "missing namespace argument",
		},
		{
			testName: "message with identifier",
			arguments: map[string]string{
				"name":      "test",
				"namespace": "apps",
			},
			matchMessage: "Kustomization test in namespace apps on the current cluster",
		},
		{
			testName: "message with cluster",
			arguments: map[string]string{
				"name":      "test",
				"namespace": "apps",
				"cluster":   "dev",
			},
			matchMessage: "Kustomization test in namespace apps on the dev cluster",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)

			request := &mcp.GetPromptRequest{
				Params: &mcp.GetPromptParams{
					Name:      "debug_flux_kustomization",
					Arguments: test.arguments,
				},
			}

			result, err := m.HandleDebugKustomization(context.Background(), request)

			if test.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(test.matchErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(result.Messages).To(HaveLen(1))
				g.Expect(result.Messages[0].Role).To(Equal(mcp.Role("assistant")))

				// In the new SDK, Content is a pointer to TextContent
				textContent, ok := result.Messages[0].Content.(*mcp.TextContent)
				g.Expect(ok).To(BeTrue())

				g.Expect(textContent.Text).To(ContainSubstring(test.matchMessage))
			}
		})
	}
}
