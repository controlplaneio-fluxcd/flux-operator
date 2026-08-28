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

// getKubernetesLogsInput defines the input parameters for retrieving pod or workload logs.
type getKubernetesLogsInput struct {
	Kind      string  `json:"kind,omitempty"      jsonschema:"Kind of the resource: Pod (default), Deployment, StatefulSet, DaemonSet, CronJob or Job."`
	Name      string  `json:"name"                jsonschema:"Name of the pod or workload."`
	Namespace string  `json:"namespace"           jsonschema:"Namespace of the pod or workload."`
	Container string  `json:"container,omitempty" jsonschema:"Container name; omit to read all containers."`
	Limit     float64 `json:"limit,omitempty"     jsonschema:"Max log lines. Defaults to 100."`
	Previous  bool    `json:"previous,omitempty"  jsonschema:"Logs from the previously terminated containers."`
}

// HandleGetKubernetesLogs is the handler function for the get_kubernetes_logs tool.
func (m *Manager) HandleGetKubernetesLogs(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesLogsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesLogs, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.Name == "" {
		return NewToolResultError("name is required")
	}
	if input.Namespace == "" {
		return NewToolResultError("namespace is required")
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

	result, err := kubeClient.GetLogs(ctx, input.Kind, input.Name, input.Namespace, input.Container, limit, input.Previous)
	if err != nil {
		return NewToolResultError(err.Error())
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(string(data))
}
