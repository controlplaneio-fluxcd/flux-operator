// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleInstallFluxInstance(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "install_flux_instance",
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName: "fails without instance_url",
			arguments: map[string]any{
				"instance_url": "",
			},
			matchErr: "The instance URL cannot be empty",
		},
		{
			testName: "fails with invalid timeout",
			arguments: map[string]any{
				"instance_url": "https://example.com/instance.yaml",
				"timeout":      "invalid",
			},
			matchErr: "The timeout is not a valid duration",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"instance_url": "https://example.com/instance.yaml",
			},
			matchErr: "failed to fetch instance manifest",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input installFluxInstanceInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleInstallFluxInstance(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}
