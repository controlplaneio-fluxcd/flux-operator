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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action struct {
	Name        string
	Description string
	Handler     any
}

var ActionList = []Action{
	{
		Name:        "reconcile-flux-source",
		Description: "This tool triggers the reconciliation of a Flux source identified by kind, name and namespace.",
		Handler:     ReconcileSourceHandler,
	},
	{
		Name:        "reconcile-flux-kustomization",
		Description: "This tool triggers the reconciliation of a Flux Kustomization identified by name and namespace.",
		Handler:     ReconcileKustomizationHandler,
	},
	{
		Name:        "reconcile-flux-helmrelease",
		Description: "This tool triggers the reconciliation of a Flux HelmRelease identified by name and namespace.",
		Handler:     ReconcileHelmReleaseHandler,
	},
}

type ReconcileSourceArgs struct {
	Kind      string `json:"kind" jsonschema:"required,description=The Flux source kind. Can only one of GitRepository, OCIRepository, Bucket, HelmChart."`
	Name      string `json:"name" jsonschema:"required,description=The name of the Flux object."`
	Namespace string `json:"namespace" jsonschema:"required,description=The namespace of the Flux object."`
}

type ReconcileKustomizationArgs struct {
	Name       string `json:"name" jsonschema:"required,description=The name of the Flux Kustomization."`
	Namespace  string `json:"namespace" jsonschema:"required,description=The namespace of the Flux Kustomization."`
	WithSource bool   `json:"withSource" jsonschema:"description=If true, the source will be reconciled as well."`
}

type ReconcileHelmReleaseArgs struct {
	Name       string `json:"name" jsonschema:"required,description=The name of the Flux HelmRelease."`
	Namespace  string `json:"namespace" jsonschema:"required,description=The namespace of the Flux HelmRelease."`
	WithSource bool   `json:"withSource" jsonschema:"description=If true, the source will be reconciled as well."`
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

	kubeClient, err := newKubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
		},
	}
	ks.SetName(args.Name)
	ks.SetNamespace(args.Namespace)

	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(ks), ks); err != nil {
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

		var rErr error
		switch sourceRefType {
		case "GitRepository":
			rErr = annotateResource(ctx,
				"source.toolkit.fluxcd.io",
				"v1",
				"GitRepository",
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case "Bucket":
			rErr = annotateResource(ctx,
				"source.toolkit.fluxcd.io",
				"v1",
				"Bucket",
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case "OCIRepository":
			rErr = annotateResource(ctx,
				"source.toolkit.fluxcd.io",
				"v1beta2",
				"OCIRepository",
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		default:
			return nil, fmt.Errorf("unknown sourceRef kind %s", sourceRefType)
		}
		if rErr != nil {
			return nil, fmt.Errorf("unable to reconcile source: %w", rErr)
		}
	}

	err = annotateResource(ctx,
		"kustomize.toolkit.fluxcd.io",
		"v1",
		"Kustomization",
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

func ReconcileHelmReleaseHandler(ctx context.Context, args ReconcileHelmReleaseArgs) (*mcpgolang.ToolResponse, error) {
	if args.Name == "" {
		return nil, errors.New("name is required")
	}
	if args.Namespace == "" {
		return nil, errors.New("namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Check if the HelmRelease exists
	kubeClient, err := newKubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client: %w", err)
	}

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
		},
	}
	hr.SetName(args.Name)
	hr.SetNamespace(args.Namespace)

	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(hr), hr); err != nil {
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

			var rErr error
			switch chartRefType {
			case "HelmChart":
				rErr = annotateResource(ctx,
					"source.toolkit.fluxcd.io",
					"v1",
					"HelmChart",
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			case "OCIRepository":
				rErr = annotateResource(ctx,
					"source.toolkit.fluxcd.io",
					"v1beta2",
					"OCIRepository",
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			default:
				return nil, fmt.Errorf("unknown chartRef kind %s", chartRefType)
			}
			if rErr != nil {
				return nil, fmt.Errorf("unable to reconcile source: %w", rErr)
			}
		}
	}

	err = annotateResource(ctx,
		"helm.toolkit.fluxcd.io",
		"v2",
		"HelmRelease",
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

	ts := time.Now().Format(time.RFC3339Nano)
	var err error
	switch {
	case strings.Contains(strings.ToLower(args.Kind), "gitrepository"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1",
			"GitRepository",
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "bucket"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1",
			"Bucket",
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "helmchart"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1",
			"HelmChart",
			args.Name,
			args.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case strings.Contains(strings.ToLower(args.Kind), "ocirepository"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1beta2",
			"OCIRepository",
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

func annotateResource(ctx context.Context, group, version, kind, name, namespace string, keys []string, val string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", kind, namespace, name, err)
	}

	patch := client.MergeFrom(resource.DeepCopy())

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	for _, key := range keys {
		annotations[key] = val
		resource.SetAnnotations(annotations)
	}

	if err := kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", kind, namespace, name, err)
	}

	return nil
}
