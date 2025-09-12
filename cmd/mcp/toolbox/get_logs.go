// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolGetKubernetesLogs is the name of the get_kubernetes_logs tool.
	ToolGetKubernetesLogs = "get_kubernetes_logs"
)

// NewGetKubernetesLogsTool creates a new tool for retrieving the pod logs.
func (m *Manager) NewGetKubernetesLogsTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolGetKubernetesLogs,
			mcp.WithDescription("This tool retrieves the the most recent logs of a Kubernetes pod."),
			mcp.WithString("pod_name",
				mcp.Description("The name of the pod."),
				mcp.Required(),
			),
			mcp.WithString("container_name",
				mcp.Description("The name of the container."),
				mcp.Required(),
			),
			mcp.WithString("pod_namespace",
				mcp.Description("The namespace of the pod."),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("The maximum number of log lines to return. Defaults to 100."),
			),
		),
		Handler:   m.HandleGetKubernetesLogs,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleGetKubernetesLogs is the handler function for the get_kubernetes_logs tool.
func (m *Manager) HandleGetKubernetesLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolGetKubernetesLogs, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	podName := mcp.ParseString(request, "pod_name", "")
	if podName == "" {
		return mcp.NewToolResultError("pod name is required"), nil
	}
	containerName := mcp.ParseString(request, "container_name", "")
	if containerName == "" {
		return mcp.NewToolResultError("container name is required"), nil
	}
	podNamespace := mcp.ParseString(request, "pod_namespace", "")
	if podNamespace == "" {
		return mcp.NewToolResultError("pod namespace is required"), nil
	}
	limit := mcp.ParseInt(request, "limit", 100)

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	result, err := kubeClient.GetLogs(ctx, podName, containerName, podNamespace, int64(limit))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed marshalling data", err), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}
