// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleGetFluxInstance(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(cli.NewConfigFlags(false)),
		flags:      cli.NewConfigFlags(false),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "get_flux_instance",
		},
	}

	tests := []struct {
		testName string
		matchErr string
	}{
		{
			testName: "fails with invalid kubeconfig",
			matchErr: "Failed to create Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)

			result, content, err := m.HandleGetFluxInstance(context.Background(), request, struct{}{})
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())

			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}
