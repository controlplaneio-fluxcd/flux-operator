// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// NewSetKubeconfigContextTool creates a new tool for setting the current kubeconfig context.
func (m *Manager) NewSetKubeconfigContextTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("set_kubeconfig_context",
			mcp.WithDescription("This tool changes the kubeconfig context for this session."),
			mcp.WithString("name",
				mcp.Description("The name of the kubeconfig context."),
				mcp.Required(),
			),
		),
		Handler:   m.HandleSetKubeconfigContext,
		ReadOnly:  true,
		InCluster: false,
	}
}

// HandleSetKubeconfigContext is the handler function for the set_kubeconfig_context tool.
func (m *Manager) HandleSetKubeconfigContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	err := m.kubeconfig.Load()
	if err != nil {
		return mcp.NewToolResultErrorFromErr("error reading kubeconfig contexts", err), nil
	}

	err = m.kubeconfig.SetCurrentContext(name)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("error setting kubeconfig context", err), nil
	}
	m.flags.Context = &name

	return mcp.NewToolResultText(fmt.Sprintf("Context changed to %s", name)), nil
}
