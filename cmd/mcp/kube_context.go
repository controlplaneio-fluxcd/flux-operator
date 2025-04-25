// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

type KubeConfigTool struct {
	Name        string
	Description string
	Handler     any
}

var KubeConfigToolList = []KubeConfigTool{
	{
		Name:        "get-kubeconfig-contexts",
		Description: "This tool retrieves the Kubernetes clusters contexts found in the kubeconfig.",
		Handler:     GetKubernetesContextHandler,
	},
	{
		Name:        "set-kubeconfig-context",
		Description: "This tool changes the kubeconfig context for this session.",
		Handler:     SetKubernetesContextHandler,
	},
}

type GetKubernetesContextArgs struct {
}

func GetKubernetesContextHandler(ctx context.Context, args GetKubernetesContextArgs) (*mcpgolang.ToolResponse, error) {
	list, err := getKubeContexts()
	if err != nil {
		return nil, fmt.Errorf("error reading kubeconfig contexts: %w", err)
	}

	data, err := yaml.Marshal(list)
	if err != nil {
		return nil, fmt.Errorf("error marshalling gvk: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(string(data))), nil
}

type SetKubernetesContextArgs struct {
	Name string `json:"name" jsonschema:"description=The name of the kubeconfig context."`
}

func SetKubernetesContextHandler(ctx context.Context, args SetKubernetesContextArgs) (*mcpgolang.ToolResponse, error) {
	kubeconfigArgs.Context = &args.Name

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(
		fmt.Sprintf(`Context changed to %s.
If asked, use the get-flux-instance tool to determine the Flux controllers status on the current cluster.
`, args.Name))), nil
}

type kubeContext struct {
	Name    string `json:"name" jsonschema:"description=The name of the kubeconfig context."`
	Current bool   `json:"current" jsonschema:"description=Whether the context is the current context."`
}

func getKubeContexts() ([]kubeContext, error) {
	kubeConfig := os.Getenv("KUBECONFIG")
	if kubeConfig == "" {
		return nil, nil
	}

	paths := filepath.SplitList(kubeConfig)

	var contexts []kubeContext
	config, err := clientcmd.LoadFromFile(paths[0])
	if err != nil {
		return nil, err
	}

	for contextName := range config.Contexts {
		kubeCtx := kubeContext{
			Name: contextName,
		}
		if contextName == config.CurrentContext {
			kubeCtx.Current = true
		}
		contexts = append(contexts, kubeCtx)
	}

	return contexts, nil
}
