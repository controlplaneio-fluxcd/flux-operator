// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleGetKubeconfigContexts(t *testing.T) {
	g := NewWithT(t)

	configFile := "testdata/kubeconfig.yaml"
	goldenFile := "testdata/kubeconfig_golden.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		flags:      cli.NewConfigFlags(false),
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: "get_kubeconfig_contexts",
		},
	}

	result, _, err := m.HandleGetKubeconfigContexts(context.Background(), request, struct{}{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeNil())
	g.Expect(result.Content).ToNot(BeEmpty())

	goldenContent, err := os.ReadFile(goldenFile)
	g.Expect(err).ToNot(HaveOccurred())

	textContent, ok := result.Content[0].(*mcp.TextContent)
	g.Expect(ok).To(BeTrue())
	g.Expect(textContent.Text).To(MatchYAML(string(goldenContent)))
}
