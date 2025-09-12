// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolGetFluxInstance is the name of the get_flux_instance tool.
	ToolGetFluxInstance = "get_flux_instance"
)

// NewGetFluxInstanceTool creates a new tool for retrieving the Flux instance report.
func (m *Manager) NewGetFluxInstanceTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolGetFluxInstance,
			mcp.WithDescription("This tool retrieves the Flux instance installation and a detailed report about Flux controllers, CRDs and their status."),
		),
		Handler:   m.HandleGetFluxInstance,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleGetFluxInstance is the handler function for the get_flux_instance tool.
func (m *Manager) HandleGetFluxInstance(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolGetFluxInstance, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	result, err := kubeClient.Export(ctx, []schema.GroupVersionKind{
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxInstanceKind,
		},
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxReportKind,
		},
	}, "", "", "", 1, true)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to determine the Flux status", err), nil
	}

	if result == "" {
		return mcp.NewToolResultError("No Flux instance found"), nil
	}

	return mcp.NewToolResultText(result), nil
}
