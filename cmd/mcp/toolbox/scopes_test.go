// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"reflect"
	"testing"
)

func TestGetToolScopes(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		readOnly bool
		expected []Scope
	}{
		{
			name:     "read-only tool with extra scope",
			tool:     ToolSearchFluxDocs,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolSearchFluxDocs,
					Description: "Allow searching the Flux documentation.",
					Tools:       []string{ToolSearchFluxDocs},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolSearchFluxDocs},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolSearchFluxDocs},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesAPIVersions",
			tool:     ToolGetKubernetesAPIVersions,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesAPIVersions,
					Description: "Allow getting available Kubernetes APIs.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
			},
		},
		{
			name:     "read-only tool GetFluxInstance",
			tool:     ToolGetFluxInstance,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetFluxInstance,
					Description: "Allow getting a FluxInstance resource.",
					Tools:       []string{ToolGetFluxInstance},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolGetFluxInstance},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetFluxInstance},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesLogs",
			tool:     ToolGetKubernetesLogs,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesLogs,
					Description: "Allow getting logs from pods.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesMetrics",
			tool:     ToolGetKubernetesMetrics,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesMetrics,
					Description: "Allow getting metrics from the Kubernetes API.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesResources",
			tool:     ToolGetKubernetesResources,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesResources,
					Description: "Allow getting Kubernetes resources.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
			},
		},
		{
			name:     "write tool without extra scopes",
			tool:     ToolApplyKubernetesManifest,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolApplyKubernetesManifest,
					Description: "Allow applying Kubernetes manifests.",
					Tools:       []string{ToolApplyKubernetesManifest},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolApplyKubernetesManifest},
				},
			},
		},
		{
			name:     "delete tool without extra scopes",
			tool:     ToolDeleteKubernetesResource,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolDeleteKubernetesResource,
					Description: "Allow deleting Kubernetes resources.",
					Tools:       []string{ToolDeleteKubernetesResource},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolDeleteKubernetesResource},
				},
			},
		},
		{
			name:     "reconcile HelmRelease tool",
			tool:     ToolReconcileFluxHelmRelease,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxHelmRelease,
					Description: "Allow reconciling HelmRelease resources.",
					Tools:       []string{ToolReconcileFluxHelmRelease},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolReconcileFluxHelmRelease},
				},
			},
		},
		{
			name:     "reconcile Kustomization tool",
			tool:     ToolReconcileFluxKustomization,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxKustomization,
					Description: "Allow reconciling Kustomization resources.",
					Tools:       []string{ToolReconcileFluxKustomization},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolReconcileFluxKustomization},
				},
			},
		},
		{
			name:     "reconcile ResourceSet tool",
			tool:     ToolReconcileFluxResourceSet,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxResourceSet,
					Description: "Allow reconciling ResourceSet resources.",
					Tools:       []string{ToolReconcileFluxResourceSet},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolReconcileFluxResourceSet},
				},
			},
		},
		{
			name:     "reconcile Source tool",
			tool:     ToolReconcileFluxSource,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxSource,
					Description: "Allow reconciling Flux source resources.",
					Tools:       []string{ToolReconcileFluxSource},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolReconcileFluxSource},
				},
			},
		},
		{
			name:     "resume reconciliation tool",
			tool:     ToolResumeFluxReconciliation,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolResumeFluxReconciliation,
					Description: "Allow resuming the reconciliation of Flux resources.",
					Tools:       []string{ToolResumeFluxReconciliation},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolResumeFluxReconciliation},
				},
			},
		},
		{
			name:     "suspend reconciliation tool",
			tool:     ToolSuspendFluxReconciliation,
			readOnly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolSuspendFluxReconciliation,
					Description: "Allow suspending the reconciliation of Flux resources.",
					Tools:       []string{ToolSuspendFluxReconciliation},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all operations.",
					Tools:       []string{ToolSuspendFluxReconciliation},
				},
			},
		},
		{
			name:     "read-only tool with readonly=true",
			tool:     ToolGetKubernetesResources,
			readOnly: true,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesResources,
					Description: "Allow getting Kubernetes resources.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
			},
		},
		{
			name:     "write tool with readonly=true",
			tool:     ToolApplyKubernetesManifest,
			readOnly: true,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolApplyKubernetesManifest,
					Description: "Allow applying Kubernetes manifests.",
					Tools:       []string{ToolApplyKubernetesManifest},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GetToolScopes(tt.tool, tt.readOnly)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("getScopes(%q, %v) = %+v, expected %+v", tt.tool, tt.readOnly, actual, tt.expected)
			}
		})
	}
}
