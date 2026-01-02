// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ToolGetKubernetesAPIVersions is the name of the get_kubernetes_api_versions tool.
	ToolGetKubernetesAPIVersions = "get_kubernetes_api_versions"
)

func init() {
	systemTools[ToolGetKubernetesAPIVersions] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// HandleGetAPIVersions is the handler function for the get_kubernetes_api_versions tool.
func (m *Manager) HandleGetAPIVersions(ctx context.Context, request *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesAPIVersions, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	result, err := kubeClient.ExportAPIs(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to export Kubernetes APIs", err)
	}

	return NewToolResultText(result)
}
