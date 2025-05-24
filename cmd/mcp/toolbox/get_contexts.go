// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"sigs.k8s.io/yaml"
)

// NewGetKubeconfigContextsTool creates a new tool for retrieving Kubernetes contexts from the kubeconfig.
func (m *Manager) NewGetKubeconfigContextsTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("get_kubeconfig_contexts",
			mcp.WithDescription("This tool retrieves the Kubernetes clusters name and context found in the kubeconfig."),
		),
		Handler:   m.HandleGetKubeconfigContexts,
		ReadOnly:  true,
		InCluster: false,
	}
}

// HandleGetKubeconfigContexts is the handler function for the get_kubeconfig_contexts tool.
func (m *Manager) HandleGetKubeconfigContexts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	err := m.kubeconfig.Load()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to read kubeconfig", err), nil
	}

	data, err := yaml.Marshal(m.kubeconfig.Contexts())
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed marshalling data", err), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}
