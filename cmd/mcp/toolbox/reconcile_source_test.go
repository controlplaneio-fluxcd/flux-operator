// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleReconcileSource(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		flags:      cli.NewConfigFlags(false),
		timeout:    time.Second,
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = "reconcile_flux_source"

	tests := []struct {
		testName  string
		arguments map[string]interface{}
		matchErr  string
	}{
		{
			testName: "fails without kind",
			arguments: map[string]interface{}{
				"name": "test",
			},
			matchErr: "kind is required",
		},
		{
			testName: "fails without name",
			arguments: map[string]interface{}{
				"kind": "Deployment",
			},
			matchErr: "name is required",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]interface{}{
				"kind": "GitRepository",
				"name": "test",
			},
			matchErr: "namespace is required",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]interface{}{
				"kind":      "OCIRepository",
				"name":      "test",
				"namespace": "default",
			},
			matchErr: "Failed to create Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			request.Params.Arguments = test.arguments

			result, err := m.HandleReconcileSource(context.Background(), request)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := mcp.AsTextContent(result.Content[0])
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
		})
	}
}
