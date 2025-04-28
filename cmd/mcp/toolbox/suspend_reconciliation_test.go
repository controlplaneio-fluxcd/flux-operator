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

func TestManager_HandleSuspendReconciliation(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		flags:      cli.NewConfigFlags(false),
		timeout:    time.Second,
	}

	request := mcp.CallToolRequest{}
	request.Params.Name = "suspend_flux_reconciliation"

	tests := []struct {
		testName  string
		arguments map[string]interface{}
		matchErr  string
	}{
		{
			testName: "fails without apiVersion",
			arguments: map[string]interface{}{
				"name": "test",
			},
			matchErr: "apiVersion is required",
		},
		{
			testName: "fails without kind",
			arguments: map[string]interface{}{
				"apiVersion": "v1",
			},
			matchErr: "kind is required",
		},
		{
			testName: "fails without name",
			arguments: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Deployment",
			},
			matchErr: "name is required",
		},
		{
			testName: "fails without namespace",
			arguments: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Deployment",
				"name":       "test",
			},
			matchErr: "namespace is required",
		},
		{
			testName: "fails with invalid kubeconfig",
			arguments: map[string]interface{}{
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
			request.Params.Arguments = test.arguments

			result, err := m.HandleSuspendReconciliation(context.Background(), request)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := mcp.AsTextContent(result.Content[0])
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
		})
	}
}
