// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type KubeConfigTool struct {
	Name        string
	Description string
	Handler     any
}

var KubeConfigToolList = []KubeConfigTool{
	{
		Name:        "get_kubeconfig_contexts",
		Description: "This tool retrieves the Kubernetes clusters contexts found in the kubeconfig.",
		Handler:     GetKubernetesContextHandler,
	},
	{
		Name:        "set_kubeconfig_context",
		Description: "This tool changes the kubeconfig context for this session.",
		Handler:     SetKubernetesContextHandler,
	},
}

var kubeconfig *client.KubeConfig

func init() {
	kubeconfig = client.NewKubeConfig()
}

type GetKubernetesContextArgs struct {
}

func GetKubernetesContextHandler(ctx context.Context, args GetKubernetesContextArgs) (*mcpgolang.ToolResponse, error) {
	err := kubeconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("error reading kubeconfig contexts: %w", err)
	}

	data, err := yaml.Marshal(kubeconfig.Contexts())
	if err != nil {
		return nil, fmt.Errorf("error marshalling gvk: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(string(data))), nil
}

type SetKubernetesContextArgs struct {
	Name string `json:"name" jsonschema:"description=The name of the kubeconfig context."`
}

func SetKubernetesContextHandler(ctx context.Context, args SetKubernetesContextArgs) (*mcpgolang.ToolResponse, error) {
	if args.Name == "" {
		return nil, fmt.Errorf("context name is required")
	}

	err := kubeconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("error reading kubeconfig contexts: %w", err)
	}

	err = kubeconfig.SetCurrentContext(args.Name)
	if err != nil {
		return nil, fmt.Errorf("error setting kubeconfig context: %w", err)
	}
	kubeconfigArgs.Context = &args.Name

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(
		fmt.Sprintf("Context changed to %s", args.Name))), nil
}
