// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"strings"
)

const (
	// ScopesPrefix is the prefix for all toolbox-related scopes.
	ScopesPrefix = "toolbox:"
	// ScopeReadOnly is a scope that allows all read-only toolbox operations.
	ScopeReadOnly = ScopesPrefix + "read_only"
	// ScopeReadWrite is a scope that allows all toolbox operations.
	ScopeReadWrite = ScopesPrefix + "read_write"
)

var scopeDescriptions = map[string]string{
	ScopeReadOnly:  "Allow all read-only operations.",
	ScopeReadWrite: "Allow all operations.",
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

	// Tools whose access are granted by this scope.
	// +optional
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
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolGetKubernetesAPIVersions: {
		ownScopeDescription: "Allow getting available Kubernetes APIs.",
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolGetFluxInstance: {
		ownScopeDescription: "Allow getting a FluxInstance resource.",
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolGetKubernetesLogs: {
		ownScopeDescription: "Allow getting logs from pods.",
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolGetKubernetesMetrics: {
		ownScopeDescription: "Allow getting metrics from the Kubernetes API.",
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolGetKubernetesResources: {
		ownScopeDescription: "Allow getting Kubernetes resources.",
		extraScopes:         []string{ScopeReadOnly},
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
		ownScopeDescription: "Allow installing Flux Operator and a FluxInstance.",
		extraScopes:         []string{},
	},
	ToolGetKubeConfigContexts: {
		ownScopeDescription: "Allow getting kubeconfig contexts.",
		extraScopes:         []string{ScopeReadOnly},
	},
	ToolSetKubeConfigContext: {
		ownScopeDescription: "Allow setting the current kubeconfig context in the MCP server memory.",
		extraScopes:         []string{ScopeReadOnly},
	},
}

// GetToolScopes returns the scopes that grant access to the given tool.
// Those are the tool-specific scope, the ScopeReadWrite scope, and
// any other extra scopes the tool can accept according to the
// static/global scopesPerTool map.
func GetToolScopes(tool string, readOnly bool) []Scope {
	ts := scopesPerTool[tool]
	scopes := make([]Scope, 0, 2+len(ts.extraScopes))
	scopes = append(scopes, Scope{ScopesPrefix + tool, ts.ownScopeDescription, []string{tool}})
	var extraScopes []string
	if !readOnly {
		extraScopes = []string{ScopeReadWrite}
	}
	for _, name := range append(extraScopes, ts.extraScopes...) {
		scopes = append(scopes, Scope{name, scopeDescriptions[name], []string{tool}})
	}
	return scopes
}

// scopesContextKey is the context key for storing scopes.
type scopesContextKey struct{}

// WithScopes adds the scopes to the given context.
func WithScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, scopesContextKey{}, scopes)
}

// CheckScopes returns an error if the context does not include
// at least one of the required scopes. If scope validation is
// disabled, nil is returned.
func CheckScopes(ctx context.Context, tool string, readOnly bool) error {
	// Get the scopes from the context.
	v := ctx.Value(scopesContextKey{})
	if v == nil {
		// Scopes are not present in the context, hence they are not enabled.
		return nil
	}
	scopes := make(map[string]struct{})
	for _, scope := range v.([]string) {
		scopes[scope] = struct{}{}
	}

	// Compute required scopes.
	toolScopes := GetToolScopes(tool, readOnly)
	requiredScopes := make([]string, 0, len(toolScopes))
	for _, s := range toolScopes {
		requiredScopes = append(requiredScopes, s.Name)
	}

	// Check if at least one required scope is present.
	for _, reqScope := range requiredScopes {
		if _, ok := scopes[reqScope]; ok {
			return nil
		}
	}

	// No required scope found.
	return fmt.Errorf("at least one of the following scopes is required: {%s}",
		strings.Join(requiredScopes, ", "))
}
