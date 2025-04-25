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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type ReconcileKustomizationArgs struct {
	Name       string `json:"name" jsonschema:"required,description=The name of the Flux Kustomization."`
	Namespace  string `json:"namespace" jsonschema:"required,description=The namespace of the Flux Kustomization."`
	WithSource bool   `json:"with_source" jsonschema:"description=If true, the source will be reconciled as well."`
}

func ReconcileKustomizationHandler(ctx context.Context, args ReconcileKustomizationArgs) (*mcpgolang.ToolResponse, error) {
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

	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
		},
	}
	ks.SetName(args.Name)
	ks.SetNamespace(args.Namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(ks), ks); err != nil {
		return nil, fmt.Errorf("unable to get Kustomization: %w", err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if args.WithSource {
		sourceRefType, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "kind")
		sourceRefName, _, _ := unstructured.NestedString(ks.Object, "spec", "chartRef", "name")
		sourceRefNamespace, _, _ := unstructured.NestedString(ks.Object, "spec", "chartRef", "namespace")
		if sourceRefNamespace == "" {
			sourceRefNamespace = args.Namespace
		}

		var err error
		switch sourceRefType {
		case "GitRepository":
			err = kubeClient.AnnotateResource(ctx,
				schema.GroupVersionKind{
					Group:   "source.toolkit.fluxcd.io",
					Version: "v1",
					Kind:    "GitRepository",
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case "Bucket":
			err = kubeClient.AnnotateResource(ctx,
				schema.GroupVersionKind{
					Group:   "source.toolkit.fluxcd.io",
					Version: "v1",
					Kind:    "Bucket",
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case "OCIRepository":
			err = kubeClient.AnnotateResource(ctx,
				schema.GroupVersionKind{
					Group:   "source.toolkit.fluxcd.io",
					Version: "v1beta2",
					Kind:    "OCIRepository",
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		default:
			return nil, fmt.Errorf("unknown sourceRef kind %s", sourceRefType)
		}
		if err != nil {
			return nil, fmt.Errorf("unable to reconcile source: %w", err)
		}
	}

	err = kubeClient.AnnotateResource(ctx,
		schema.GroupVersionKind{
			Group:   "kustomize.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Kustomization",
		},
		args.Name,
		args.Namespace,
		[]string{meta.ReconcileRequestAnnotation},
		ts)

	if err != nil {
		return nil, fmt.Errorf("unable to reconcile Kustomization: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(`Kustomization reconciliation triggered, 
to verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)), nil
}
