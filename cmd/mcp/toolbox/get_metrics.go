// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/yaml"
)

const (
	// ToolGetKubernetesMetrics is the name of the get_kubernetes_metrics tool.
	ToolGetKubernetesMetrics = "get_kubernetes_metrics"
)

func init() {
	systemTools[ToolGetKubernetesMetrics] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// getKubernetesMetricsInput defines the input parameters for retrieving pod metrics.
type getKubernetesMetricsInput struct {
	PodName      string  `json:"pod_name,omitempty" jsonschema:"The name of the pod when not specified all pods are selected."`
	PodNamespace string  `json:"pod_namespace" jsonschema:"The namespace of the pods."`
	PodSelector  string  `json:"pod_selector,omitempty" jsonschema:"The pod label selector in the format key1=value1 key2=value2."`
	Limit        float64 `json:"limit,omitempty" jsonschema:"The maximum number of resources to return. Defaults to 100."`
}

// HandleGetKubernetesMetrics is the handler function for the get_kubernetes_metrics tool.
func (m *Manager) HandleGetKubernetesMetrics(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesMetricsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesMetrics, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.PodNamespace == "" {
		return NewToolResultError("pod namespace is required")
	}
	limit := int(input.Limit)
	if limit == 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	result, err := kubeClient.GetMetrics(ctx, input.PodName, input.PodNamespace, input.PodSelector, limit)
	if err != nil {
		return NewToolResultError(err.Error())
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(string(data))
}
