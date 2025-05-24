// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewGetFluxInstanceTool creates a new tool for retrieving the Flux instance report.
func (m *Manager) NewGetFluxInstanceTool() SystemTool {
	return SystemTool{
		mcp.NewTool("get_flux_instance",
			mcp.WithDescription("This tool retrieves the Flux instance installation and a detailed report about Flux controllers, CRDs and their status."),
		),
		m.HandleGetFluxInstance,
		true,
		true, // InCluster is true because it operates on the cluster's Flux instance.
	}
}

// HandleGetFluxInstance is the handler function for the get_flux_instance tool.
func (m *Manager) HandleGetFluxInstance(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
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
