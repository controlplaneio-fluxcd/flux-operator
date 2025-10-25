// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

const (
	// scopesPrefix is the prefix for all toolbox-related scopes.
	scopesPrefix = "toolbox:"
	// scopeReadOnly is a scope that allows all read-only toolbox operations.
	scopeReadOnly = scopesPrefix + "read_only"
	// scopeReadWrite is a scope that allows all toolbox operations.
	scopeReadWrite = scopesPrefix + "read_write"
)

var scopeDescriptions = map[string]string{
	scopeReadOnly:  "Allow all read-only toolbox operations.",
	scopeReadWrite: "Allow all toolbox operations.",
}

// Scope represents a scope with its metadata.
type Scope struct {
	// Name is the scope identifier.
	// +required
	Name string `json:"name"`
	// Description is a brief, human-readable description
	// of the scope for a permission consent screen. It
	// should not refer to the scope name or as a concept,
	// but rather give a high-level description of what
	// the scope allows.
	// +required
	Description string `json:"description"`
	// Tools are tools whose access are granted by this scope.
	// Not necessarily all of them, but at least one.
	// +required
	Tools []string `json:"tools,omitempty"`
}

// toolScopes defines the set of scopes that can grant access
// to a tool. Any scope alone in this set is sufficient to
// grant access to the tool.
type toolScopes struct {
	// ownScopeDescription is the description of the tool-specific scope.
	ownScopeDescription string
	// extraScopes are additional scopes the tool can accept.
	extraScopes []string
}

// scopesPerTool defines the set of scopes that can grant access
// to each tool. The obvious scopes (derived in GetScopes) are not
// repeated here.
var scopesPerTool = map[string]toolScopes{
	ToolSearchFluxDocs: {
		ownScopeDescription: "Allow searching the Flux documentation.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolGetKubernetesAPIVersions: {
		ownScopeDescription: "Allow getting available Kubernetes APIs.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolGetFluxInstance: {
		ownScopeDescription: "Allow getting a FluxInstance resource.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolGetKubernetesLogs: {
		ownScopeDescription: "Allow getting logs from pods.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolGetKubernetesMetrics: {
		ownScopeDescription: "Allow getting metrics from the Kubernetes API.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolGetKubernetesResources: {
		ownScopeDescription: "Allow getting Kubernetes resources.",
		extraScopes:         []string{scopeReadOnly},
	},
	ToolApplyKubernetesManifest: {
		ownScopeDescription: "Allow applying Kubernetes manifests.",
		extraScopes:         []string{},
	},
	ToolDeleteKubernetesResource: {
		ownScopeDescription: "Allow deleting Kubernetes resources.",
		extraScopes:         []string{},
	},
	ToolReconcileFluxHelmRelease: {
		ownScopeDescription: "Allow reconciling HelmRelease resources.",
		extraScopes:         []string{},
	},
	ToolReconcileFluxKustomization: {
		ownScopeDescription: "Allow reconciling Kustomization resources.",
		extraScopes:         []string{},
	},
	ToolReconcileFluxResourceSet: {
		ownScopeDescription: "Allow reconciling ResourceSet resources.",
		extraScopes:         []string{},
	},
	ToolReconcileFluxSource: {
		ownScopeDescription: "Allow reconciling Flux source resources.",
		extraScopes:         []string{},
	},
	ToolResumeFluxReconciliation: {
		ownScopeDescription: "Allow resuming the reconciliation of Flux resources.",
		extraScopes:         []string{},
	},
	ToolSuspendFluxReconciliation: {
		ownScopeDescription: "Allow suspending the reconciliation of Flux resources.",
		extraScopes:         []string{},
	},
	ToolInstallFluxInstance: {
		ownScopeDescription: "Allow installing Flux Operator and Flux instance.",
		extraScopes:         []string{},
	},
}

// getScopes returns the scopes that grant access to the given tool.
// Those are the tool-specific scope, the ScopeReadWrite scope, and
// any other extra scopes the tool can accept according to the
// static/global scopesPerTool map.
func getScopes(tool string, readOnly bool) []Scope {
	ts := scopesPerTool[tool]
	scopes := make([]Scope, 0, 2+len(ts.extraScopes))
	scopes = append(scopes, Scope{scopesPrefix + tool, ts.ownScopeDescription, []string{tool}})
	var extraScopes []string
	if !readOnly {
		extraScopes = []string{scopeReadWrite}
	}
	for _, name := range append(extraScopes, ts.extraScopes...) {
		scopes = append(scopes, Scope{name, scopeDescriptions[name], []string{tool}})
	}
	return scopes
}

// getScopeNames returns the names of the scopes that grant access to
// the given tool according to GetScopes.
func getScopeNames(tool string, readOnly bool) []string {
	scopes := getScopes(tool, readOnly)
	scopeNames := make([]string, 0, len(scopes))
	for _, s := range scopes {
		scopeNames = append(scopeNames, s.Name)
	}
	return scopeNames
}

// AddScopesAndFilter adds to the ListToolsResult metadata the scopes
// granting access to each tool in the MCP server, and filters out tools
// that the user session does not have access to based on the scopes
// present in the provided context. If the context is nil, no filtering
// is done.
func AddScopesAndFilter(ctx context.Context, result *mcp.ListToolsResult, readOnly bool) {
	// Sweep tools accumulating scopes and filtering out the tools
	// that the user session does not have access to.
	// The scopes are accumulated in a map to avoid duplicates.
	scopesMap := make(map[string]*Scope)
	scopeIndex := map[string]int{
		// Extra scopes should come first in the output.
		scopeReadOnly:  -2,
		scopeReadWrite: -1,
	}
	var filteredTools []*mcp.Tool
	for idx, t := range result.Tools {
		toolScopes := getScopes(t.Name, readOnly)
		toolScopeNames := make([]string, 0, len(toolScopes))
		for _, ts := range toolScopes {
			scope, ok := scopesMap[ts.Name]
			if !ok {
				scopesMap[ts.Name] = &ts
			} else {
				scope.Tools = append(scope.Tools, t.Name)
			}
			if ts.Name == scopesPrefix+t.Name {
				scopeIndex[ts.Name] = idx
			}
			toolScopeNames = append(toolScopeNames, ts.Name)
		}
		if ctx == nil {
			filteredTools = append(filteredTools, t)
			continue
		}
		if err := auth.CheckScopes(ctx, toolScopeNames); err == nil {
			filteredTools = append(filteredTools, t)
		}
	}

	// Convert the scopes map to a slice and sort according to the
	// original order of the tools, with the extra scopes first.
	scopes := make([]*Scope, 0, len(scopesMap))
	for _, scope := range scopesMap {
		scopes = append(scopes, scope)
	}
	slices.SortFunc(scopes, func(a, b *Scope) int {
		return scopeIndex[a.Name] - scopeIndex[b.Name]
	})

	// Add the scopes to the result metadata and update the tools.
	if result.Meta == nil {
		result.Meta = make(map[string]any)
	}
	result.Meta["scopes"] = scopes
	result.Tools = filteredTools
}
