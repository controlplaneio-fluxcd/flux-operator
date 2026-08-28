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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleGetKubernetesEvents(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(genericclioptions.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: ToolGetKubernetesEvents,
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName:  "rejects invalid type before loading client",
			arguments: map[string]any{"type": "Error"},
			matchErr:  "type must be Normal or Warning",
		},
		{
			testName:  "rejects invalid since before loading client",
			arguments: map[string]any{"since": "recently"},
			matchErr:  "Invalid since:",
		},
		{
			testName:  "rejects non-positive since before loading client",
			arguments: map[string]any{"since": "-5m"},
			matchErr:  "Invalid since: must be a positive duration",
		},
		{
			testName:  "rejects invalid grep before loading client",
			arguments: map[string]any{"grep": "["},
			matchErr:  "Invalid grep:",
		},
		{
			testName:  "accepts optional filters before loading client",
			arguments: map[string]any{"type": "Warning", "since": "10m", "grep": "BackOff"},
			matchErr:  "Failed to get Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input getKubernetesEventsInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleGetKubernetesEvents(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())
			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}

func TestGetKubernetesEventsOutput(t *testing.T) {
	g := NewWithT(t)
	result := &k8s.ListEventsResult{
		Events: "2026-08-28T10:15:02Z Warning BackOff Pod/apps/backend-7f9d4c-abcde x17 Back-off restarting failed container\n" +
			"2026-08-28T10:14:31Z Warning Unhealthy Pod/apps/backend-7f9d4c-abcde Readiness probe failed: HTTP status 503\n",
		Namespace: "apps",
		Total:     12,
		Truncated: false,
	}

	data, err := yaml.Marshal(result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(Equal(`events: |
  2026-08-28T10:15:02Z Warning BackOff Pod/apps/backend-7f9d4c-abcde x17 Back-off restarting failed container
  2026-08-28T10:14:31Z Warning Unhealthy Pod/apps/backend-7f9d4c-abcde Readiness probe failed: HTTP status 503
namespace: apps
total: 12
truncated: false
`))
}
