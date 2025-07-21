// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Apply parses the YAML manifest and creates or updates the Kubernetes objects using server-side apply.
// If any of the Kubernetes objects are managed by Flux, it will return an error unless overwrite is set to true.
func (k *Client) Apply(ctx context.Context, manifest string, overwrite bool) (string, error) {
	objects, err := ssautil.ReadObjects(strings.NewReader(manifest))
	if err != nil {
		return "", fmt.Errorf("unable to parse YAML manifest: %w", err)
	}

	if len(objects) == 0 {
		return "", fmt.Errorf("no Kubernetes objects found in manifest")
	}

	if !overwrite {
		for _, object := range objects {
			if k.IsManagedByFlux(ctx, object.GroupVersionKind(), object.GetName(), object.GetNamespace()) {
				return "", fmt.Errorf("%s/%s is managed by Flux",
					object.GetKind(), object.GetName())
			}
		}
	}

	err = normalize.UnstructuredList(objects)
	if err != nil {
		return "", fmt.Errorf("unable to normalize objects: %w", err)
	}

	changeSet, err := k.rm.ApplyAllStaged(ctx, objects, ssa.DefaultApplyOptions())
	if err != nil {
		return "", fmt.Errorf("unable to apply objects: %w", err)
	}

	return changeSet.String(), nil
}

// IsManagedByFlux checks if a Kubernetes resource is managed by Flux by inspecting specific Flux-related labels.
func (k *Client) IsManagedByFlux(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string) bool {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return false
	}

	fluxLabels := []string{
		"fluxcd.controlplane.io/namespace",
		"resourceset.fluxcd.controlplane.io/namespace",
		fluxcdv1.FluxKustomizeGroup + "/namespace",
		fluxcdv1.FluxHelmGroup + "/namespace",
	}

	for key := range resource.GetLabels() {
		if slices.Contains(fluxLabels, key) {
			return true
		}
	}

	return false
}

// Annotate sets annotations on a Kubernetes resource identified by GroupVersionKind, name, and namespace.
func (k *Client) Annotate(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, keys []string, val string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := ctrlclient.MergeFrom(resource.DeepCopy())

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	for _, key := range keys {
		annotations[key] = val
		resource.SetAnnotations(annotations)
	}

	if err := k.Client.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// Delete deletes a Kubernetes resource identified by GroupVersionKind, name, and namespace.
func (k *Client) Delete(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	if err := k.Client.Delete(ctx, resource); err != nil {
		return fmt.Errorf("unable to delete %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// ToggleSuspension toggles the suspension of a Flux resource by updating the spec.suspend field.
func (k *Client) ToggleSuspension(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, suspend bool) error {
	if strings.EqualFold(gvk.Group, fluxcdv1.GroupVersion.Group) {
		val := fluxcdv1.EnabledValue
		if suspend {
			val = fluxcdv1.DisabledValue
		}
		return k.Annotate(ctx,
			gvk,
			name,
			namespace,
			[]string{fluxcdv1.ReconcileAnnotation},
			val)
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := ctrlclient.MergeFrom(resource.DeepCopy())

	if suspend {
		err := unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend")
		if err != nil {
			return fmt.Errorf("unable to set suspend field: %w", err)
		}
	} else {
		unstructured.RemoveNestedField(resource.Object, "spec", "suspend")
	}

	if err := k.Client.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to patch %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}
