// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	mcpgolang "github.com/metoro-io/mcp-golang"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type ReconcileSourceArgs struct {
	Kind      string `json:"kind" jsonschema:"required,description=The Flux source kind. Can only one of GitRepository, OCIRepository, Bucket, HelmChart."`
	Name      string `json:"name" jsonschema:"required,description=The name of the Flux object."`
	Namespace string `json:"namespace" jsonschema:"required,description=The namespace of the Flux object."`
}

func ReconcileSourceHandler(ctx context.Context, args ReconcileSourceArgs) (*mcpgolang.ToolResponse, error) {
	if args.Kind == "" {
		return nil, errors.New("kind is required")
	}
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

	ts := time.Now().Format(time.RFC3339Nano)
	switch {
	case strings.Contains(strings.ToLower(args.Kind), "gitrepository"):
		err = kubeClient.AnnotateResource(ctx,
			schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1",
				Kind:    "GitRepository",
			},
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "bucket"):
		err = kubeClient.AnnotateResource(ctx,
			schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1",
				Kind:    "Bucket",
			},
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "helmchart"):
		err = kubeClient.AnnotateResource(ctx,
			schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1",
				Kind:    "HelmChart",
			},
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "ocirepository"):
		err = kubeClient.AnnotateResource(ctx,
			schema.GroupVersionKind{
				Group:   "source.toolkit.fluxcd.io",
				Version: "v1beta2",
				Kind:    "OCIRepository",
			},
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	default:
		return nil, fmt.Errorf("unknown source kind %s", args.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to annotate source: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(`Source reconciliation triggered, 
to verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)), nil
}
