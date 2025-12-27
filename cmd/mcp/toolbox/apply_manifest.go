// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ToolApplyKubernetesManifest is the name of the apply_kubernetes_manifest tool.
	ToolApplyKubernetesManifest = "apply_kubernetes_manifest"
)

func init() {
	systemTools[ToolApplyKubernetesManifest] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// applyKubernetesManifestInput defines the input parameters for applying a Kubernetes manifest.
type applyKubernetesManifestInput struct {
	YAMLContent string `json:"yaml_content" jsonschema:"The multi-doc YAML content."`
	Overwrite   bool   `json:"overwrite,omitempty" jsonschema:"Overwrite resources managed by Flux."`
}

// HandleApplyKubernetesManifest is the handler function for the apply_kubernetes_manifest tool.
func (m *Manager) HandleApplyKubernetesManifest(ctx context.Context, request *mcp.CallToolRequest, input applyKubernetesManifestInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolApplyKubernetesManifest, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.YAMLContent == "" {
		return NewToolResultError("YAML manifest cannot be empty")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	changeSet, err := kubeClient.Apply(ctx, input.YAMLContent, input.Overwrite)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to apply manifest", err)
	}

	return NewToolResultText(changeSet)
}
