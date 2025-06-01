// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewGetKubernetesMetricsTool creates a new tool for retrieving pod metrics.
func (m *Manager) NewGetKubernetesMetricsTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("get_kubernetes_metrics",
			mcp.WithDescription("This tool retrieves CPU and Memory usage of Kubernetes pods."),
			mcp.WithString("pod_name",
				mcp.Description("The name of the pod, when not specified all pods are selected."),
			),
			mcp.WithString("pod_namespace",
				mcp.Description("The namespace of the pods."),
				mcp.Required(),
			),
			mcp.WithString("pod_selector",
				mcp.Description("The pod label selector in the format key1=value1,key2=value2."),
			),
			mcp.WithNumber("limit",
				mcp.Description("The maximum number of resources to return. Defaults to 100."),
			),
		),
		Handler:   m.HandleGetKubernetesMetrics,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleGetKubernetesMetrics is the handler function for the get_kubernetes_metrics tool.
func (m *Manager) HandleGetKubernetesMetrics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	podName := mcp.ParseString(request, "pod_name", "")
	podNamespace := mcp.ParseString(request, "pod_namespace", "")
	if podNamespace == "" {
		return mcp.NewToolResultError("pod namespace is required"), nil
	}
	selector := mcp.ParseString(request, "pod_selector", "")
	limit := mcp.ParseInt(request, "limit", 100)

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	result, err := kubeClient.GetMetrics(ctx, podName, podNamespace, selector, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed marshalling data", err), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}
