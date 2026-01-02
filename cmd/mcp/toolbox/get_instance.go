// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// ToolGetFluxInstance is the name of the get_flux_instance tool.
	ToolGetFluxInstance = "get_flux_instance"
)

func init() {
	systemTools[ToolGetFluxInstance] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// HandleGetFluxInstance is the handler function for the get_flux_instance tool.
func (m *Manager) HandleGetFluxInstance(ctx context.Context, request *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetFluxInstance, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
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
		return NewToolResultErrorFromErr("Failed to determine the Flux status", err)
	}

	if result == "" {
		return NewToolResultError("No Flux instance found")
	}

	return NewToolResultText(result)
}
