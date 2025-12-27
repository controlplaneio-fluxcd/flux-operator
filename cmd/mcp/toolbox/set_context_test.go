// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		kubeClient: k8s.NewClientFactory(flags),
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "set_kubeconfig_context",
		},
	}

	tests := []struct {
		testName    string
		arguments   map[string]any
		matchErr    string
		matchResult string
	}{
		{
			testName: "fails with not found context",
			arguments: map[string]any{
				"name": "test",
			},
			matchErr: "not found",
		},
		{
			testName: "changes context to staging",
			arguments: map[string]any{
				"name": "kind-staging",
			},
			matchResult: "Context changed to kind-staging",
		},
		{
			testName: "changes context to dev",
			arguments: map[string]any{
				"name": "kind-dev",
			},
			matchResult: "Context changed to kind-dev",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input setKubeconfigContextInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, _, err := m.HandleSetKubeconfigContext(context.Background(), request, input)
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
				g.Expect(textContent.Text).To(ContainSubstring(test.matchResult))
				g.Expect(*flags.Context).To(Equal(test.arguments["name"]))
			}
		})
	}
}
