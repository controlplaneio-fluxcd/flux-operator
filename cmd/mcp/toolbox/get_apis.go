// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolGetKubernetesAPIVersions is the name of the get_kubernetes_api_versions tool.
	ToolGetKubernetesAPIVersions = "get_kubernetes_api_versions"
)

// NewGetAPIVersionsTool creates a new tool for retrieving Kubernetes API versions.
func (m *Manager) NewGetAPIVersionsTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolGetKubernetesAPIVersions,
			mcp.WithDescription("This tool retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind."),
		),
		Handler:   m.HandleGetAPIVersions,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleGetAPIVersions is the handler function for the get_kubernetes_api_versions tool.
func (m *Manager) HandleGetAPIVersions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolGetKubernetesAPIVersions, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	result, err := kubeClient.ExportAPIs(ctx)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to export Kubernetes APIs", err), nil
	}

	return mcp.NewToolResultText(result), nil
}
