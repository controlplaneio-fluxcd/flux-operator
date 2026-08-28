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

func TestManager_HandleGetKubernetesLogs(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "get_kubernetes_logs",
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName: "fails without name",
			arguments: map[string]any{
				"kind": "Pod",
			},
			matchErr: "name is required",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]any{
				"name": "test",
			},
			matchErr: "namespace is required",
		},
		{
			testName: "fails with invalid since",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
				"since":     "soon",
			},
			matchErr: "Invalid since",
		},
		{
			testName: "fails with non-positive since",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
				"since":     "-5m",
			},
			matchErr: "Invalid since: must be a positive duration",
		},
		{
			testName: "fails with invalid grep",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
				"grep":      "(",
			},
			matchErr: "Invalid grep",
		},
		{
			testName: "accepts since and grep before loading client",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
				"since":     "10m",
				"grep":      "error|panic",
			},
			matchErr: "Failed to get Kubernetes client",
		},
		{
			testName: "accepts omitted kind and container before loading client",
			arguments: map[string]any{
				"name":      "test",
				"namespace": "default",
			},
			matchErr: "Failed to get Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input getKubernetesLogsInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleGetKubernetesLogs(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}
