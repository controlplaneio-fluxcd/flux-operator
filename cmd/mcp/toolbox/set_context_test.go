// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleSetKubeconfigContext(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	flags := cli.NewConfigFlags(false)
	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		flags:      flags,
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = "set_kubeconfig_context"

	tests := []struct {
		testName    string
		arguments   map[string]interface{}
		matchErr    string
		matchResult string
	}{
		{
			testName: "fails with not found context",
			arguments: map[string]interface{}{
				"name": "test",
			},
			matchErr: "not found",
		},
		{
			testName: "changes context to staging",
			arguments: map[string]interface{}{
				"name": "kind-staging",
			},
			matchResult: "Context changed to kind-staging",
		},
		{
			testName: "changes context to dev",
			arguments: map[string]interface{}{
				"name": "kind-dev",
			},
			matchResult: "Context changed to kind-dev",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			request.Params.Arguments = test.arguments

			result, err := m.HandleSetKubeconfigContext(context.Background(), request)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := mcp.AsTextContent(result.Content[0])
			g.Expect(ok).To(BeTrue())

			if test.matchErr != "" {
				g.Expect(result.IsError).To(BeTrue())
				g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			} else {
				g.Expect(result.IsError).ToNot(BeTrue())
				g.Expect(textContent.Text).To(ContainSubstring(test.matchResult))
				g.Expect(*flags.Context).To(Equal(test.arguments["name"]))
			}
		})
	}
}
