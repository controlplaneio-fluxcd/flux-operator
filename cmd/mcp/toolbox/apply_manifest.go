// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewApplyKubernetesManifestTool creates a new tool for applying Kubernetes manifests.
func (m *Manager) NewApplyKubernetesManifestTool() SystemTool {
	return SystemTool{
		mcp.NewTool("apply_kubernetes_manifest",
			mcp.WithDescription("This tool applies a Kubernetes YAML manifest on the cluster."),
			mcp.WithString("yaml",
				mcp.Description("The multi-doc YAML content."),
				mcp.Required(),
			),
		),
		m.HandleApplyKubernetesManifest,
		false,
	}
}

// HandleApplyKubernetesManifest is the handler function for the apply_kubernetes_manifest tool.
func (m *Manager) HandleApplyKubernetesManifest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	manifest := mcp.ParseString(request, "yaml", "")
	if manifest == "" {
		return mcp.NewToolResultError("YAML manifest cannot be empty"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	changeSet, err := kubeClient.Apply(ctx, manifest)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to apply manifest", err), nil
	}

	return mcp.NewToolResultText(changeSet), nil
}
