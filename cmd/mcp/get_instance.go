// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func GetFluxInstanceHandler(ctx context.Context, args GetFluxResourceArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Name, args.Namespace, args.LabelSelector, []metav1.GroupVersionKind{
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
	})
	if err != nil {
		return nil, fmt.Errorf("unable to determine the Flux status on this cluster: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}
