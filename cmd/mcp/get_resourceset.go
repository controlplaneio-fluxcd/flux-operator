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

func GetFluxResourceSetsHandler(ctx context.Context, args GetArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetKind,
		},
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetInputProviderKind,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}
