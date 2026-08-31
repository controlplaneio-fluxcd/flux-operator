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

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

func TestManager_HandleTraceKubernetesResource(t *testing.T) {
	configFile := "testdata/kubeconfig.yaml"
	t.Setenv("KUBECONFIG", configFile)

	m := &Manager{
		kubeconfig: k8s.NewKubeConfig(),
		kubeClient: k8s.NewClientFactory(genericclioptions.NewConfigFlags(false)),
		timeout:    time.Second,
	}

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name: ToolTraceKubernetesResource,
		},
	}

	tests := []struct {
		testName  string
		arguments map[string]any
		matchErr  string
	}{
		{
			testName:  "rejects missing apiVersion before loading client",
			arguments: map[string]any{"kind": "Pod", "name": "backend"},
			matchErr:  "apiVersion is required",
		},
		{
			testName:  "rejects missing kind before loading client",
			arguments: map[string]any{"apiVersion": "v1", "name": "backend"},
			matchErr:  "kind is required",
		},
		{
			testName:  "rejects missing name before loading client",
			arguments: map[string]any{"apiVersion": "v1", "kind": "Pod"},
			matchErr:  "name is required",
		},
		{
			testName:  "accepts valid input before loading client",
			arguments: map[string]any{"apiVersion": "v1", "kind": "Pod", "name": "backend", "namespace": "apps"},
			matchErr:  "Failed to get Kubernetes client",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			g := NewWithT(t)
			argsJSON, _ := json.Marshal(test.arguments)
			request.Params.Arguments = argsJSON

			var input traceKubernetesResourceInput
			err := json.Unmarshal(request.Params.Arguments, &input)
			g.Expect(err).ToNot(HaveOccurred())
			result, content, err := m.HandleTraceKubernetesResource(context.Background(), request, input)
			g.Expect(err).ToNot(HaveOccurred())
			textContent, ok := result.Content[0].(*mcp.TextContent)
			g.Expect(ok).To(BeTrue())
			g.Expect(result.IsError).To(BeTrue())
			g.Expect(textContent.Text).To(ContainSubstring(test.matchErr))
			_ = content
		})
	}
}

func TestTraceKubernetesResourceOutput(t *testing.T) {
	g := NewWithT(t)
	result := &k8s.TraceResult{
		Object: "Pod/apps/backend-7f9d4c-abcde",
		ManagedBy: &k8s.TraceManager{
			TraceLink: k8s.TraceLink{Object: "Deployment/apps/backend", Status: "Current"},
			ManagedBy: &k8s.TraceManager{
				TraceLink: k8s.TraceLink{
					Object: "HelmRelease/apps/backend", Status: "UpgradeFailed",
					Message: "Helm upgrade failed", LastAppliedRevision: "6.9.0",
				},
			},
		},
		Source: &k8s.TraceSource{
			ResolvedFor: "HelmRelease/apps/backend",
			TraceLink: k8s.TraceLink{
				Object: "OCIRepository/flux-system/apps-repo", Status: "Succeeded",
				URL: "oci://ghcr.io/org/apps", Revision: "latest@sha256:abc123",
			},
		},
	}

	data, err := marshalTraceResult(result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(data).To(Equal(`object: Pod/apps/backend-7f9d4c-abcde
managedBy:
  object: Deployment/apps/backend
  status: Current
  managedBy:
    object: HelmRelease/apps/backend
    status: UpgradeFailed
    message: Helm upgrade failed
    lastAppliedRevision: 6.9.0
source:
  resolvedFor: HelmRelease/apps/backend
  object: OCIRepository/flux-system/apps-repo
  status: Succeeded
  url: oci://ghcr.io/org/apps
  revision: latest@sha256:abc123
`))
}
