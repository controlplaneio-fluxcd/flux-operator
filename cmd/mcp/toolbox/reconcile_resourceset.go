// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxResourceSet is the name of the reconcile_flux_resourceset tool.
	ToolReconcileFluxResourceSet = "reconcile_flux_resourceset"
)

// NewReconcileResourceSetTool creates a new tool for reconciling a Flux ResourceSet.
func (m *Manager) NewReconcileResourceSetTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolReconcileFluxResourceSet,
			mcp.WithDescription("This tool triggers the reconciliation of a Flux ResourceSet."),
			mcp.WithString("name",
				mcp.Description("The name of the ResourceSet."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the ResourceSet."),
				mcp.Required(),
			),
		),
		Handler:   m.HandleReconcileResourceSet,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleReconcileResourceSet is the handler function for the reconcile_flux_resourceset tool.
func (m *Manager) HandleReconcileResourceSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolReconcileFluxResourceSet, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetKind,
		},
		name,
		namespace,
		[]string{meta.ReconcileRequestAnnotation},
		time.Now().Format(time.RFC3339Nano))

	if err != nil {
		return nil, fmt.Errorf("unable to reconcile ResourceSet: %w", err)
	}

	return mcp.NewToolResultText(`ResourceSet reconciliation triggered.
to verify check the status lastHandledReconcileAt field matches the requestedAt annotation`), nil
}
