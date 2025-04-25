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
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
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

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	err = kubeClient.AnnotateResource(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetKind,
		},
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
