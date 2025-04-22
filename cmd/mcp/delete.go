// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DeleteTool struct {
	Name        string
	Description string
	Handler     any
}

var DeleteToolList = []DeleteTool{
	{
		Name:        "delete-kubernetes-resource",
		Description: "This tool deletes a Kubernetes resource identified by apiVersion, kind, name and namespace.",
		Handler:     DeleteKubernetesResourceHandler,
	},
}

func deleteResource(ctx context.Context, apiVersion, kind, name, namespace string) error {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return fmt.Errorf("unable to parse group version %s error: %w", apiVersion, err)
	}

	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	})
	resource.SetName(name)
	resource.SetNamespace(namespace)

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Delete(ctx, resource); err != nil {
		return fmt.Errorf("unable to delete %s/%s/%s error: %w", kind, namespace, name, err)
	}

	return nil
}
