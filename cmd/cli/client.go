// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func newKubeClient() (client.Client, error) {
	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	// bump limits
	cfg.QPS = 100.0
	cfg.Burst = 300

	restMapper, err := kubeconfigArgs.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	scheme := apiruntime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := fluxcdv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return client.WithFieldOwner(kubeClient, "flux-operator-ctl"), nil
}

func annotateResource(ctx context.Context, kind, name, namespace, key, val string) error {
	return annotateResourceWithMap(ctx, kind, name, namespace, map[string]string{key: val})
}

func annotateResourceWithMap(ctx context.Context, kind, name, namespace string, m map[string]string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   fluxcdv1.GroupVersion.Group,
		Version: fluxcdv1.GroupVersion.Version,
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
	maps.Copy(annotations, m)
	resource.SetAnnotations(annotations)

	if err := kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", kind, namespace, name, err)
	}

	return nil
}
