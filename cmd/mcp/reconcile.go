// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcileTool struct {
	Name        string
	Description string
	Handler     any
}

var ReconcileToolList = []ReconcileTool{
	{
		Name:        "reconcile-flux-resourceset",
		Description: "This tool triggers the reconciliation of a Flux ResourceSet identified by name and namespace.",
		Handler:     ReconcileResourceSetHandler,
	},
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
