// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewDeleteKubernetesResourceTool creates a new tool for deleting Kubernetes resources.
func (m *Manager) NewDeleteKubernetesResourceTool() SystemTool {
	return SystemTool{
		mcp.NewTool("delete_kubernetes_resource",
			mcp.WithDescription("This tool deletes a Kubernetes resource based on its API version, kind, name, and namespace."),
			mcp.WithString("apiVersion",
				mcp.Description("The apiVersion of the resource to delete."),
				mcp.Required(),
			),
			mcp.WithString("kind",
				mcp.Description("The kind of the resource to delete."),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("The name of the resource to delete."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the resource to delete."),
			),
		),
		m.HandleDeleteKubernetesResource,
		false,
		true,
	}
}

// HandleDeleteKubernetesResource is the handler function for the delete_kubernetes_resource tool.
func (m *Manager) HandleDeleteKubernetesResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiVersion := mcp.ParseString(request, "apiVersion", "")
	if apiVersion == "" {
		return mcp.NewToolResultError("apiVersion is required"), nil
	}
	kind := mcp.ParseString(request, "kind", "")
	if kind == "" {
		return mcp.NewToolResultError("kind is required"), nil
	}
	name := mcp.ParseString(request, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	namespace := mcp.ParseString(request, "namespace", "")

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	gvk, err := kubeClient.ParseGroupVersionKind(apiVersion, kind)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to parse group version kind", err), nil
	}

	err = kubeClient.Delete(ctx, gvk, name, namespace)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to delete resource", err), nil
	}

	return mcp.NewToolResultText("Resource deleted successfully"), nil
}
