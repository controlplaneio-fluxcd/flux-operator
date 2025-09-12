// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"reflect"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

func TestGetScopes(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		readonly bool
		expected []Scope
	}{
		{
			name:     "read-only tool with extra scope",
			tool:     ToolSearchFluxDocs,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolSearchFluxDocs,
					Description: "Allow searching the Flux documentation.",
					Tools:       []string{ToolSearchFluxDocs},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolSearchFluxDocs},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolSearchFluxDocs},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesAPIVersions",
			tool:     ToolGetKubernetesAPIVersions,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesAPIVersions,
					Description: "Allow getting available Kubernetes APIs.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetKubernetesAPIVersions},
				},
			},
		},
		{
			name:     "read-only tool GetFluxInstance",
			tool:     ToolGetFluxInstance,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetFluxInstance,
					Description: "Allow getting a FluxInstance resource.",
					Tools:       []string{ToolGetFluxInstance},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolGetFluxInstance},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetFluxInstance},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesLogs",
			tool:     ToolGetKubernetesLogs,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesLogs,
					Description: "Allow getting logs from pods.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetKubernetesLogs},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesMetrics",
			tool:     ToolGetKubernetesMetrics,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesMetrics,
					Description: "Allow getting metrics from the Kubernetes API.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetKubernetesMetrics},
				},
			},
		},
		{
			name:     "read-only tool GetKubernetesResources",
			tool:     ToolGetKubernetesResources,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesResources,
					Description: "Allow getting Kubernetes resources.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
			},
		},
		{
			name:     "write tool without extra scopes",
			tool:     ToolApplyKubernetesManifest,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolApplyKubernetesManifest,
					Description: "Allow applying Kubernetes manifests.",
					Tools:       []string{ToolApplyKubernetesManifest},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolApplyKubernetesManifest},
				},
			},
		},
		{
			name:     "delete tool without extra scopes",
			tool:     ToolDeleteKubernetesResource,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolDeleteKubernetesResource,
					Description: "Allow deleting Kubernetes resources.",
					Tools:       []string{ToolDeleteKubernetesResource},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolDeleteKubernetesResource},
				},
			},
		},
		{
			name:     "reconcile HelmRelease tool",
			tool:     ToolReconcileFluxHelmRelease,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxHelmRelease,
					Description: "Allow reconciling HelmRelease resources.",
					Tools:       []string{ToolReconcileFluxHelmRelease},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolReconcileFluxHelmRelease},
				},
			},
		},
		{
			name:     "reconcile Kustomization tool",
			tool:     ToolReconcileFluxKustomization,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxKustomization,
					Description: "Allow reconciling Kustomization resources.",
					Tools:       []string{ToolReconcileFluxKustomization},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolReconcileFluxKustomization},
				},
			},
		},
		{
			name:     "reconcile ResourceSet tool",
			tool:     ToolReconcileFluxResourceSet,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxResourceSet,
					Description: "Allow reconciling ResourceSet resources.",
					Tools:       []string{ToolReconcileFluxResourceSet},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolReconcileFluxResourceSet},
				},
			},
		},
		{
			name:     "reconcile Source tool",
			tool:     ToolReconcileFluxSource,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolReconcileFluxSource,
					Description: "Allow reconciling Flux source resources.",
					Tools:       []string{ToolReconcileFluxSource},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolReconcileFluxSource},
				},
			},
		},
		{
			name:     "resume reconciliation tool",
			tool:     ToolResumeFluxReconciliation,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolResumeFluxReconciliation,
					Description: "Allow resuming the reconciliation of Flux resources.",
					Tools:       []string{ToolResumeFluxReconciliation},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolResumeFluxReconciliation},
				},
			},
		},
		{
			name:     "suspend reconciliation tool",
			tool:     ToolSuspendFluxReconciliation,
			readonly: false,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolSuspendFluxReconciliation,
					Description: "Allow suspending the reconciliation of Flux resources.",
					Tools:       []string{ToolSuspendFluxReconciliation},
				},
				{
					Name:        "toolbox:read_write",
					Description: "Allow all toolbox operations.",
					Tools:       []string{ToolSuspendFluxReconciliation},
				},
			},
		},
		{
			name:     "read-only tool with readonly=true",
			tool:     ToolGetKubernetesResources,
			readonly: true,
			expected: []Scope{
				{
					Name:        "toolbox:" + ToolGetKubernetesResources,
					Description: "Allow getting Kubernetes resources.",
					Tools:       []string{ToolGetKubernetesResources},
				},
				{
					Name:        "toolbox:read_only",
					Description: "Allow all read-only toolbox operations.",
					Tools:       []string{ToolGetKubernetesResources},
				},
			},
		},
		{
			name:     "write tool with readonly=true",
			tool:     ToolApplyKubernetesManifest,
			readonly: true,
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
			actual := getScopes(tt.tool, tt.readonly)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("getScopes(%q, %v) = %+v, expected %+v", tt.tool, tt.readonly, actual, tt.expected)
			}
		})
	}
}

func TestGetScopeNames(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		expected []string
	}{
		{
			name: "read-only tool with extra scope",
			tool: ToolSearchFluxDocs,
			expected: []string{
				"toolbox:" + ToolSearchFluxDocs,
				"toolbox:read_write",
				"toolbox:read_only",
			},
		},
		{
			name: "write tool without extra scopes",
			tool: ToolApplyKubernetesManifest,
			expected: []string{
				"toolbox:" + ToolApplyKubernetesManifest,
				"toolbox:read_write",
			},
		},
		{
			name: "delete tool",
			tool: ToolDeleteKubernetesResource,
			expected: []string{
				"toolbox:" + ToolDeleteKubernetesResource,
				"toolbox:read_write",
			},
		},
		{
			name: "get APIs tool",
			tool: ToolGetKubernetesAPIVersions,
			expected: []string{
				"toolbox:" + ToolGetKubernetesAPIVersions,
				"toolbox:read_write",
				"toolbox:read_only",
			},
		},
		{
			name: "get resources tool",
			tool: ToolGetKubernetesResources,
			expected: []string{
				"toolbox:" + ToolGetKubernetesResources,
				"toolbox:read_write",
				"toolbox:read_only",
			},
		},
		{
			name: "reconcile HelmRelease tool",
			tool: ToolReconcileFluxHelmRelease,
			expected: []string{
				"toolbox:" + ToolReconcileFluxHelmRelease,
				"toolbox:read_write",
			},
		},
		{
			name: "suspend reconciliation tool",
			tool: ToolSuspendFluxReconciliation,
			expected: []string{
				"toolbox:" + ToolSuspendFluxReconciliation,
				"toolbox:read_write",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getScopeNames(tt.tool, false)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("getScopeNames(%q) = %v, expected %v", tt.tool, actual, tt.expected)
			}
		})
	}
}

func TestAddScopesAndFilter(t *testing.T) {
	tests := []struct {
		name           string
		tools          []mcp.Tool
		contextScopes  []string
		expectedTools  []string
		expectedScopes int
		hasContext     bool
	}{
		{
			name: "no context - all tools included",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
			},
			hasContext:     false,
			expectedTools:  []string{ToolSearchFluxDocs, ToolApplyKubernetesManifest, ToolGetKubernetesResources},
			expectedScopes: 5, // 3 tool-specific + read_only + read_write (deduplicated)
		},
		{
			name: "context with read_only scope",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
			},
			hasContext:     true,
			contextScopes:  []string{"toolbox:read_only"},
			expectedTools:  []string{ToolSearchFluxDocs, ToolGetKubernetesResources},
			expectedScopes: 5,
		},
		{
			name: "context with read_write scope",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
			},
			hasContext:     true,
			contextScopes:  []string{"toolbox:read_write"},
			expectedTools:  []string{ToolSearchFluxDocs, ToolApplyKubernetesManifest, ToolGetKubernetesResources},
			expectedScopes: 5,
		},
		{
			name: "context with specific tool scope",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
			},
			hasContext:     true,
			contextScopes:  []string{"toolbox:" + ToolApplyKubernetesManifest},
			expectedTools:  []string{ToolApplyKubernetesManifest},
			expectedScopes: 5,
		},
		{
			name: "context with multiple scopes",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
				{Name: ToolDeleteKubernetesResource},
			},
			hasContext: true,
			contextScopes: []string{
				"toolbox:read_only",
				"toolbox:" + ToolDeleteKubernetesResource,
			},
			expectedTools:  []string{ToolSearchFluxDocs, ToolGetKubernetesResources, ToolDeleteKubernetesResource},
			expectedScopes: 6,
		},
		{
			name:           "empty tools list",
			tools:          []mcp.Tool{},
			hasContext:     false,
			expectedTools:  []string{},
			expectedScopes: 0,
		},
		{
			name: "context with no matching scopes",
			tools: []mcp.Tool{
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolDeleteKubernetesResource},
			},
			hasContext:     true,
			contextScopes:  []string{"invalid:scope"},
			expectedTools:  []string{},
			expectedScopes: 3,
		},
		{
			name: "all write tools with read_only scope",
			tools: []mcp.Tool{
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolDeleteKubernetesResource},
				{Name: ToolReconcileFluxHelmRelease},
				{Name: ToolSuspendFluxReconciliation},
			},
			hasContext:     true,
			contextScopes:  []string{"toolbox:read_only"},
			expectedTools:  []string{},
			expectedScopes: 5,
		},
		{
			name: "mix of read and write tools with various scopes",
			tools: []mcp.Tool{
				{Name: ToolGetKubernetesLogs},
				{Name: ToolGetKubernetesMetrics},
				{Name: ToolReconcileFluxKustomization},
				{Name: ToolReconcileFluxResourceSet},
				{Name: ToolResumeFluxReconciliation},
			},
			hasContext: true,
			contextScopes: []string{
				"toolbox:" + ToolGetKubernetesLogs,
				"toolbox:" + ToolReconcileFluxKustomization,
			},
			expectedTools:  []string{ToolGetKubernetesLogs, ToolReconcileFluxKustomization},
			expectedScopes: 7,
		},
		{
			name: "scope ordering verification",
			tools: []mcp.Tool{
				{Name: ToolSearchFluxDocs},
				{Name: ToolApplyKubernetesManifest},
				{Name: ToolGetKubernetesResources},
				{Name: ToolDeleteKubernetesResource},
			},
			hasContext:     false,
			expectedTools:  []string{ToolSearchFluxDocs, ToolApplyKubernetesManifest, ToolGetKubernetesResources, ToolDeleteKubernetesResource},
			expectedScopes: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &mcp.ListToolsResult{
				Tools: tt.tools,
			}

			var ctx context.Context
			if tt.hasContext {
				ctx = context.Background()
				if len(tt.contextScopes) > 0 {
					session := &auth.Session{
						Scopes: tt.contextScopes,
					}
					ctx = auth.IntoContext(ctx, session)
				}
			}

			AddScopesAndFilter(ctx, result, false)

			// Check filtered tools
			actualToolNames := make([]string, len(result.Tools))
			for i, tool := range result.Tools {
				actualToolNames[i] = tool.Name
			}
			if !reflect.DeepEqual(actualToolNames, tt.expectedTools) {
				t.Errorf("filtered tools = %v, expected %v", actualToolNames, tt.expectedTools)
			}

			// Check metadata was added
			if result.Meta == nil || result.Meta.AdditionalFields == nil {
				if tt.expectedScopes > 0 {
					t.Error("expected metadata to be added but it was nil")
				}
				return
			}

			// Check scopes in metadata
			scopesInterface, ok := result.Meta.AdditionalFields["scopes"]
			if !ok {
				if tt.expectedScopes > 0 {
					t.Error("expected scopes in metadata but not found")
				}
				return
			}

			scopes, ok := scopesInterface.([]*Scope)
			if !ok {
				t.Error("scopes in metadata has wrong type")
				return
			}

			if len(scopes) != tt.expectedScopes {
				t.Errorf("number of scopes = %d, expected %d", len(scopes), tt.expectedScopes)
			}

			// Verify scopes are sorted correctly (read_only first, then read_write, then tool-specific)
			if len(scopes) > 0 {
				// Check that read_only comes before read_write if both present
				readOnlyIdx := -1
				readWriteIdx := -1
				for i, scope := range scopes {
					if scope.Name == "toolbox:read_only" {
						readOnlyIdx = i
					}
					if scope.Name == "toolbox:read_write" {
						readWriteIdx = i
					}
				}
				if readOnlyIdx != -1 && readWriteIdx != -1 && readOnlyIdx > readWriteIdx {
					t.Error("read_only scope should come before read_write scope")
				}

				// Check that read_only and read_write are at the beginning if they exist
				if readOnlyIdx >= 0 || readWriteIdx >= 0 {
					expectedStartIdx := 0
					if readOnlyIdx >= 0 {
						if scopes[expectedStartIdx].Name != "toolbox:read_only" {
							t.Errorf("read_only scope should be at index %d, but found at %d", expectedStartIdx, readOnlyIdx)
						}
						expectedStartIdx++
					}
					if readWriteIdx >= 0 {
						if scopes[expectedStartIdx].Name != "toolbox:read_write" {
							t.Errorf("read_write scope should be at index %d, but found at %d", expectedStartIdx, readWriteIdx)
						}
					}
				}
			}
		})
	}
}
