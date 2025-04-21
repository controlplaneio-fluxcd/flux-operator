// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	mcpgolang "github.com/metoro-io/mcp-golang"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type ReconcileResourceSetArgs struct {
	Name      string `json:"name" jsonschema:"required,description=The name of the ResourceSet."`
	Namespace string `json:"namespace" jsonschema:"required,description=The namespace of the ResourceSet."`
}

func ReconcileResourceSetHandler(ctx context.Context, args ReconcileResourceSetArgs) (*mcpgolang.ToolResponse, error) {
	if args.Name == "" {
		return nil, errors.New("name is required")
	}
	if args.Namespace == "" {
		return nil, errors.New("namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := annotateResource(ctx,
		fluxcdv1.GroupVersion.Group,
		fluxcdv1.GroupVersion.Version,
		fluxcdv1.ResourceSetKind,
		args.Name,
		args.Namespace,
		[]string{meta.ReconcileRequestAnnotation},
		time.Now().Format(time.RFC3339Nano))

	if err != nil {
		return nil, fmt.Errorf("unable to reconcile ResourceSet: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(`ResourceSet reconciliation triggered, 
to verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)), nil
}
