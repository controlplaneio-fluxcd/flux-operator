// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type DeleteKubernetesResourceArgs struct {
	ApiVersion string `json:"apiVersion" jsonschema:"required,description=The apiVersion of the resource to delete."`
	Kind       string `json:"kind" jsonschema:"required,description=The kind of the resource to delete."`
	Name       string `json:"name" jsonschema:"required,description=The name of the resource to delete."`
	Namespace  string `json:"namespace" jsonschema:"description=The namespace of the resource to delete."`
}

func DeleteKubernetesResourceHandler(ctx context.Context, args DeleteKubernetesResourceArgs) (*mcpgolang.ToolResponse, error) {
	if args.ApiVersion == "" {
		return nil, fmt.Errorf("apiVersion is required")
	}
	if args.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}
	if args.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	gvk, err := kubeClient.ParseGroupVersionKind(args.ApiVersion, args.Kind)
	if err != nil {
		return nil, fmt.Errorf("unable to parse group version kind %s/%s: %w", args.ApiVersion, args.Kind, err)
	}

	err = kubeClient.DeleteResource(ctx, gvk, args.Name, args.Namespace)
	if err != nil {
		return nil, fmt.Errorf("unable to delete resource: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent("Resource deleted successfully")), nil
}
