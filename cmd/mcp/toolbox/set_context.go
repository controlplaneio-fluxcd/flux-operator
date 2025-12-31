// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ToolSetKubeConfigContext is the name of the set_kubeconfig_context tool.
	ToolSetKubeConfigContext = "set_kubeconfig_context"
)

func init() {
	systemTools[ToolSetKubeConfigContext] = systemTool{
		readOnly:  true,
		inCluster: false,
	}
}

// setKubeconfigContextInput defines the input parameters for setting the kubeconfig context.
type setKubeconfigContextInput struct {
	Name string `json:"name" jsonschema:"The name of the kubeconfig context."`
}

// HandleSetKubeconfigContext is the handler function for the set_kubeconfig_context tool.
func (m *Manager) HandleSetKubeconfigContext(ctx context.Context, request *mcp.CallToolRequest, input setKubeconfigContextInput) (*mcp.CallToolResult, any, error) {
	if input.Name == "" {
		return NewToolResultError("name is required")
	}

	err := m.kubeconfig.SetCurrentContext(input.Name)
	if err != nil {
		return NewToolResultErrorFromErr("error setting kubeconfig context", err)
	}
	m.flags.Context = &input.Name

	return NewToolResultText(fmt.Sprintf("Context changed to %s", input.Name))
}
