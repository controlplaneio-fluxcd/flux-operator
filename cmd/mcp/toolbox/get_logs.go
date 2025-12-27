// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/yaml"
)

const (
	// ToolGetKubernetesLogs is the name of the get_kubernetes_logs tool.
	ToolGetKubernetesLogs = "get_kubernetes_logs"
)

func init() {
	systemTools[ToolGetKubernetesLogs] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// getKubernetesLogsInput defines the input parameters for retrieving pod logs.
type getKubernetesLogsInput struct {
	PodName       string  `json:"pod_name" jsonschema:"The name of the pod."`
	ContainerName string  `json:"container_name" jsonschema:"The name of the container."`
	PodNamespace  string  `json:"pod_namespace" jsonschema:"The namespace of the pod."`
	Limit         float64 `json:"limit,omitempty" jsonschema:"The maximum number of log lines to return. Defaults to 100."`
}

// HandleGetKubernetesLogs is the handler function for the get_kubernetes_logs tool.
func (m *Manager) HandleGetKubernetesLogs(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesLogsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesLogs, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.PodName == "" {
		return NewToolResultError("pod name is required")
	}
	if input.ContainerName == "" {
		return NewToolResultError("container name is required")
	}
	if input.PodNamespace == "" {
		return NewToolResultError("pod namespace is required")
	}
	limit := int64(input.Limit)
	if limit == 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	result, err := kubeClient.GetLogs(ctx, input.PodName, input.ContainerName, input.PodNamespace, limit)
	if err != nil {
		return NewToolResultError(err.Error())
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(string(data))
}
