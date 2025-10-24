// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package prompter

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewDebugKustomizationPrompt creates a prompt for debugging lux Kustomization.
func (m *Manager) NewDebugKustomizationPrompt() SystemPrompt {
	return SystemPrompt{
		Prompt: &mcp.Prompt{
			Name:        "debug_flux_kustomization",
			Description: "",
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "name",
					Description: "The name of the Flux resource.",
					Required:    true,
				},
				{
					Name:        "namespace",
					Description: "The namespace of the Flux resource.",
					Required:    true,
				},
				{
					Name:        "cluster",
					Description: "The context name of the cluster, defaults to current.",
					Required:    false,
				},
			},
		},
		Handler: m.HandleDebugKustomization,
	}
}

// HandleDebugKustomization is the handler function for the debug_flux_kustomization prompt.
func (m *Manager) HandleDebugKustomization(ctx context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	name := request.Params.Arguments["name"]
	if name == "" {
		return nil, fmt.Errorf("missing name argument")
	}
	namespace := request.Params.Arguments["namespace"]
	if namespace == "" {
		return nil, fmt.Errorf("missing namespace argument")
	}
	cluster := request.Params.Arguments["cluster"]
	if cluster == "" {
		cluster = "current"
	}

	return &mcp.GetPromptResult{
		Description: "Debug instructions for a Kustomization",
		Messages: []*mcp.PromptMessage{
			{
				Role: "assistant",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf(`
To debug the Flux Kustomization %[1]s in namespace %[2]s on the %[3]s cluster, follow these steps:

0. Use the get_kubeconfig_contexts tool to find the context name for the %[3]s cluster
and use the set_kubeconfig_context to change the context to it.
1. Use the get_flux_instance tool to check the kustomize-controller deployment
status and the available Flux API versions.
2. Retrieve the Kustomization details using the get_kubernetes_resources tool.
3. Identify the Kustomization source by looking at the spec.sourceRef field,
then use the get_kubernetes_resources tool to fetch the corresponding
OCIRepository, Bucket, or GitRepository resource.
4. If the Kustomization is in a failed state or in progress, check the status
conditions and events for any error messages.
5. Use the get_kubernetes_resources tool to check the status of the resources
found in the Kustomization status inventory.
6. Write a detailed report of the issue, including the Kustomization spec, status, and any error messages.
`, name, namespace, cluster),
				},
			},
		},
	}, nil
}
