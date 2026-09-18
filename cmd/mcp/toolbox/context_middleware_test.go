// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// multiClusterKubeconfig holds two token-authenticated contexts, so clients
// can be built for each of them without reaching a cluster.
const multiClusterKubeconfig = `apiVersion: v1
kind: Config
clusters:
  - cluster:
      server: https://dev.example.com:6443
      insecure-skip-tls-verify: true
    name: kind-dev
  - cluster:
      server: https://staging.example.com:6443
      insecure-skip-tls-verify: true
    name: kind-staging
contexts:
  - context:
      cluster: kind-dev
      user: kind-dev
    name: kind-dev
  - context:
      cluster: kind-staging
      user: kind-staging
    name: kind-staging
current-context: kind-dev
users:
  - name: kind-dev
    user:
      token: dev
  - name: kind-staging
    user:
      token: staging
`

func TestManager_ContextMiddleware(t *testing.T) {
	g := NewWithT(t)

	kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
	g.Expect(os.WriteFile(kubeconfig, []byte(multiClusterKubeconfig), 0o600)).To(Succeed())
	flags := cli.NewConfigFlags(false)
	flags.KubeConfig = &kubeconfig
	m := NewManager(k8s.NewClientFactory(flags), 0, false, true, false, true)

	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		matchHost string
		matchErr  string
	}{
		{
			name:      "runs against the named context",
			tool:      ToolGetFluxInstance,
			arguments: map[string]any{"context": "kind-staging"},
			matchHost: "https://staging.example.com:6443",
		},
		{
			name:      "keeps the current context without the input",
			tool:      ToolGetFluxInstance,
			arguments: map[string]any{},
			matchHost: "https://dev.example.com:6443",
		},
		{
			name:      "ignores the input of tools that do not talk to a cluster",
			tool:      ToolSearchFluxDocs,
			arguments: map[string]any{"context": "kind-staging"},
			matchHost: "https://dev.example.com:6443",
		},
		{
			name:      "fails for an unknown context",
			tool:      ToolGetFluxInstance,
			arguments: map[string]any{"context": "kind-prod"},
			matchErr:  "kind-prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var handlerCtx context.Context
			next := func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				handlerCtx = ctx
				return &mcp.CallToolResult{}, nil
			}

			arguments, err := json.Marshal(tt.arguments)
			g.Expect(err).ToNot(HaveOccurred())
			req := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{Name: tt.tool, Arguments: arguments},
			}

			result, err := m.ContextMiddleware()(next)(context.Background(), "tools/call", req)
			g.Expect(err).ToNot(HaveOccurred())

			if tt.matchErr != "" {
				g.Expect(handlerCtx).To(BeNil())
				callResult, ok := result.(*mcp.CallToolResult)
				g.Expect(ok).To(BeTrue())
				g.Expect(callResult.IsError).To(BeTrue())
				g.Expect(callResult.Content[0].(*mcp.TextContent).Text).To(ContainSubstring(tt.matchErr))
				return
			}

			g.Expect(handlerCtx).ToNot(BeNil())
			kubeClient, err := m.kubeClient.GetClient(handlerCtx)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(kubeClient.GetConfig().Host).To(Equal(tt.matchHost))
		})
	}
}
