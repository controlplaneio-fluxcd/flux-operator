// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/yaml"
)

const (
	// ToolGetKubeConfigContexts is the name of the get_kubeconfig_contexts tool.
	ToolGetKubeConfigContexts = "get_kubeconfig_contexts"
)

func init() {
	systemTools[ToolGetKubeConfigContexts] = systemTool{
		readOnly:  true,
		inCluster: false,
	}
}

// HandleGetKubeconfigContexts is the handler function for the get_kubeconfig_contexts tool.
func (m *Manager) HandleGetKubeconfigContexts(ctx context.Context, request *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
	err := m.kubeconfig.Load()
	if err != nil {
		return NewToolResultErrorFromErr("Failed to read kubeconfig", err)
	}

	data, err := yaml.Marshal(m.kubeconfig.Contexts())
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(string(data))
}
