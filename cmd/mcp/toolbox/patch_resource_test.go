// Copyright 2026 Stefan Prodan.
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

func TestManager_HandlePatchKubernetesResource(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(cli.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: ToolPatchKubernetesResource},
	}

	tests := []struct {
		name      string
		arguments map[string]any
		matchErr  string
	}{
		{
			name:      "fails without apiVersion",
			arguments: map[string]any{"kind": "ConfigMap", "name": "test", "patch": "data: {}"},
			matchErr:  "apiVersion is required",
		},
		{
			name:      "fails without kind",
			arguments: map[string]any{"apiVersion": "v1", "name": "test", "patch": "data: {}"},
			matchErr:  "kind is required",
		},
		{
			name:      "fails without name",
			arguments: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "patch": "data: {}"},
			matchErr:  "name is required",
		},
		{
			name:      "fails without patch",
			arguments: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "test"},
			matchErr:  "patch is required",
		},
		{
			name: "fails with bad type",
			arguments: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap", "name": "test", "patch": "data: {}", "type": "apply",
			},
			matchErr: "type must be merge, json or strategic",
		},
		{
			name: "fails with bad subresource",
			arguments: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap", "name": "test", "patch": "data: {}", "subresource": "scale",
			},
			matchErr: "subresource must be status",
		},
		{
			name: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap", "name": "test", "patch": "data: {}",
			},
			matchErr: "Failed to get Kubernetes client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, err := json.Marshal(tt.arguments)
			g.Expect(err).NotTo(HaveOccurred())
			request.Params.Arguments = argsJSON

			var input patchKubernetesResourceInput
			g.Expect(json.Unmarshal(request.Params.Arguments, &input)).To(Succeed())
			result, _, err := m.HandlePatchKubernetesResource(context.Background(), request, input)
			g.Expect(err).NotTo(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())
			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(tt.matchErr))
		})
	}
}
