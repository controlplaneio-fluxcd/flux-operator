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

type ReconcileHelmReleaseArgs struct {
	Name       string `json:"name" jsonschema:"required,description=The name of the Flux HelmRelease."`
	Namespace  string `json:"namespace" jsonschema:"required,description=The namespace of the Flux HelmRelease."`
	WithSource bool   `json:"with_source" jsonschema:"description=If true, the source will be reconciled as well."`
}

func ReconcileHelmReleaseHandler(ctx context.Context, args ReconcileHelmReleaseArgs) (*mcpgolang.ToolResponse, error) {
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

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
		},
	}
	hr.SetName(args.Name)
	hr.SetNamespace(args.Namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(hr), hr); err != nil {
		return nil, fmt.Errorf("unable to get HelmRelease: %w", err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if args.WithSource {
		chartRefType, found, err := unstructured.NestedString(hr.Object, "spec", "chartRef", "kind")
		if found && err == nil {
			chartRefName, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "name")
			chartRefNamespace, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "namespace")
			if chartRefNamespace == "" {
				chartRefNamespace = args.Namespace
			}

			var err error
			switch chartRefType {
			case "HelmChart":
				err = kubeClient.AnnotateResource(ctx,
					schema.GroupVersionKind{
						Group:   "source.toolkit.fluxcd.io",
						Version: "v1",
						Kind:    "HelmChart",
					},
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			case "OCIRepository":
				err = kubeClient.AnnotateResource(ctx,
					schema.GroupVersionKind{
						Group:   "source.toolkit.fluxcd.io",
						Version: "v1beta2",
						Kind:    "OCIRepository",
					},
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			default:
				return nil, fmt.Errorf("unknown chartRef kind %s", chartRefType)
			}
			if err != nil {
				return nil, fmt.Errorf("unable to reconcile source: %w", err)
			}
		}
	}

	err = kubeClient.AnnotateResource(ctx,
		schema.GroupVersionKind{
			Group:   "helm.toolkit.fluxcd.io",
			Version: "v2",
			Kind:    "HelmRelease",
		},
		args.Name,
		args.Namespace,
		[]string{
			meta.ReconcileRequestAnnotation,
			"reconcile.fluxcd.io/forceAt",
		},
		ts)

	if err != nil {
		return nil, fmt.Errorf("unable to reconcile HelmRelease: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(`HelmRelease reconciliation triggered, 
to verify check the status lastHandledForceAt field matches the forceAt annotation`)), nil
}
