// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolGetKubernetesResources is the name of the get_kubernetes_resources tool.
	ToolGetKubernetesResources = "get_kubernetes_resources"
)

func init() {
	systemTools[ToolGetKubernetesResources] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// getKubernetesResourcesInput defines the input parameters for retrieving Kubernetes resources.
type getKubernetesResourcesInput struct {
	APIVersion string   `json:"apiVersion" jsonschema:"apiVersion of the resource. Use get_kubernetes_api_versions when unknown."`
	Kind       string   `json:"kind" jsonschema:"Kind of the resource."`
	Name       string   `json:"name,omitempty" jsonschema:"Name of the resource; omit to list."`
	Namespace  string   `json:"namespace,omitempty" jsonschema:"Namespace; omit for all namespaces."`
	Selector   string   `json:"selector,omitempty" jsonschema:"Label selector in the format key1=value1 key2=value2."`
	Limit      float64  `json:"limit,omitempty" jsonschema:"Max number of resources to return."`
	Fields     []string `json:"fields,omitempty" jsonschema:"kubectl JSONPath expressions to include in the result to reduce its size, e.g. spec.sourceRef.name or status.conditions[?(@.type=='Ready')].message."`
}

// HandleGetKubernetesResources is the handler function for the get_kubernetes_resources tool.
func (m *Manager) HandleGetKubernetesResources(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesResourcesInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesResources, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.APIVersion == "" {
		return NewToolResultError("apiVersion is required")
	}
	if input.Kind == "" {
		return NewToolResultError("kind is required")
	}
	limit := int(input.Limit)

	fieldPaths, err := k8s.ParseFieldPaths(input.Fields)
	if err != nil {
		return NewToolResultErrorFromErr("Invalid fields", err)
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

	result, err := kubeClient.Export(ctx,
		[]schema.GroupVersionKind{gvk},
		input.Name,
		input.Namespace,
		input.Selector,
		limit,
		m.maskSecrets,
		fieldPaths)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to export resources", err)
	}

	if result == "" {
		return NewToolResultError("No resources found")
	}

	return NewToolResultText(result)
}
