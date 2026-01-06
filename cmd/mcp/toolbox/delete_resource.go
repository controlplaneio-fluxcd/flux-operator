// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// ToolDeleteKubernetesResource is the name of the delete_kubernetes_resource tool.
	ToolDeleteKubernetesResource = "delete_kubernetes_resource"
)

func init() {
	systemTools[ToolDeleteKubernetesResource] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// deleteKubernetesResourceInput defines the input parameters for deleting a Kubernetes resource.
type deleteKubernetesResourceInput struct {
	APIVersion string `json:"apiVersion" jsonschema:"The apiVersion of the resource to delete."`
	Kind       string `json:"kind" jsonschema:"The kind of the resource to delete."`
	Name       string `json:"name" jsonschema:"The name of the resource to delete."`
	Namespace  string `json:"namespace,omitempty" jsonschema:"The namespace of the resource to delete."`
}

// HandleDeleteKubernetesResource is the handler function for the delete_kubernetes_resource tool.
func (m *Manager) HandleDeleteKubernetesResource(ctx context.Context, request *mcp.CallToolRequest, input deleteKubernetesResourceInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolDeleteKubernetesResource, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.APIVersion == "" {
		return NewToolResultError("apiVersion is required")
	}
	if input.Kind == "" {
		return NewToolResultError("kind is required")
	}
	if input.Name == "" {
		return NewToolResultError("name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	gvk, err := kubeClient.ParseGroupVersionKind(input.APIVersion, input.Kind)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to parse group version kind", err)
	}

	err = kubeClient.Delete(ctx, gvk, input.Name, input.Namespace)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to delete resource", err)
	}

	return NewToolResultText("Resource deleted successfully")
}
