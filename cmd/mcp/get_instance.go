// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type GetFluxInstanceArgs struct {
	Name      string `json:"name" jsonschema:"description=Filter by a specific name."`
	Namespace string `json:"namespace" jsonschema:"description=Filter by a specific namespace, if not specified all namespaces are included."`
}

func GetFluxInstanceHandler(ctx context.Context, args GetFluxInstanceArgs) (*mcpgolang.ToolResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	result, err := kubeClient.Export(ctx, []schema.GroupVersionKind{
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxInstanceKind,
		},
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxReportKind,
		},
	}, args.Name, args.Namespace, "", true)
	if err != nil {
		return nil, fmt.Errorf("unable to determine the Flux status on this cluster: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}
