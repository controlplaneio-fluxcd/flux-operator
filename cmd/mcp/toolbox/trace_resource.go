// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"bytes"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	yamlv3 "go.yaml.in/yaml/v3"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolTraceKubernetesResource is the name of the trace_kubernetes_resource tool.
	ToolTraceKubernetesResource = "trace_kubernetes_resource"
)

func init() {
	systemTools[ToolTraceKubernetesResource] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// traceKubernetesResourceInput defines the input parameters for tracing a Kubernetes resource.
type traceKubernetesResourceInput struct {
	APIVersion string `json:"apiVersion" jsonschema:"apiVersion of the resource. Use get_kubernetes_api_versions when unknown."`
	Kind       string `json:"kind" jsonschema:"Kind of the resource."`
	Name       string `json:"name" jsonschema:"Name of the resource."`
	Namespace  string `json:"namespace,omitempty" jsonschema:"Namespace of the resource; omit for cluster-scoped resources."`
}

// HandleTraceKubernetesResource is the handler function for the trace_kubernetes_resource tool.
func (m *Manager) HandleTraceKubernetesResource(ctx context.Context, request *mcp.CallToolRequest, input traceKubernetesResourceInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolTraceKubernetesResource, m.readOnly); err != nil {
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

	result, err := kubeClient.Trace(ctx, k8s.TraceOptions{
		APIVersion: input.APIVersion,
		Kind:       input.Kind,
		Name:       input.Name,
		Namespace:  input.Namespace,
	})
	if err != nil {
		return NewToolResultError(err.Error())
	}

	data, err := marshalTraceResult(result)
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(data)
}

// marshalTraceResult renders the trace result as YAML,
// preserving the field order the structs declare.
func marshalTraceResult(result *k8s.TraceResult) (string, error) {
	var buf bytes.Buffer
	encoder := yamlv3.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(result); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}
