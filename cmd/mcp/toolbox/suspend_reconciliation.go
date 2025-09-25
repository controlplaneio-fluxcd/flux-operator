// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolSuspendFluxReconciliation is the name of the suspend_flux_reconciliation tool.
	ToolSuspendFluxReconciliation = "suspend_flux_reconciliation"
)

// NewSuspendReconciliationTool creates a new tool for suspending the reconciliation of a Flux resource.
func (m *Manager) NewSuspendReconciliationTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolSuspendFluxReconciliation,
			mcp.WithDescription("This tool suspends the reconciliation of a Flux resource."),
			mcp.WithString("apiVersion",
				mcp.Description("The apiVersion of the Flux resource."),
				mcp.Required(),
			),
			mcp.WithString("kind",
				mcp.Description("The kind of the Flux resource."),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("The name of the Flux resource."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the Flux resource."),
				mcp.Required(),
			),
		),
		Handler:   m.HandleSuspendReconciliation,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleSuspendReconciliation is the handler function for the suspend_flux_reconciliation tool.
func (m *Manager) HandleSuspendReconciliation(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolSuspendFluxReconciliation, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	apiVersion := mcp.ParseString(request, "apiVersion", "")
	if apiVersion == "" {
		return mcp.NewToolResultError("apiVersion is required"), nil
	}
	kind := mcp.ParseString(request, "kind", "")
	if kind == "" {
		return mcp.NewToolResultError("kind is required"), nil
	}
	name := mcp.ParseString(request, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	namespace := mcp.ParseString(request, "namespace", "")
	if namespace == "" {
		return mcp.NewToolResultError("namespace is required"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	gvk, err := kubeClient.ParseGroupVersionKind(apiVersion, kind)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to parse group version kind", err), nil
	}

	err = kubeClient.ToggleSuspension(ctx, gvk, name, namespace, true)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to suspend reconciliation", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Reconciliation of %s/%s/%s suspended", gvk.Kind, namespace, name)), nil
}
