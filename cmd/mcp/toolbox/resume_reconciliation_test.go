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

func TestManager_HandleResumeReconciliation(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		flags:      cli.NewConfigFlags(false),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "resume_flux_reconciliation",
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName: "fails without apiVersion",
			arguments: map[string]any{
				"name": "test",
			},
			matchErr: "apiVersion is required",
		},
		{
			testName: "fails without kind",
			arguments: map[string]any{
				"apiVersion": "v1",
			},
			matchErr: "kind is required",
		},
		{
			testName: "fails without name",
			arguments: map[string]any{
				"apiVersion": "v1",
				"kind":       "Deployment",
			},
			matchErr: "name is required",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]any{
				"apiVersion": "v1",
				"kind":       "Deployment",
				"name":       "test",
			},
			matchErr: "namespace is required",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       "test",
				"namespace":  "default",
			},
			matchErr: "Failed to create Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input resumeFluxReconciliationInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleResumeReconciliation(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}
