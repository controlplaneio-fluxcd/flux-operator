// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/cli-utils/pkg/object"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

// InventoryEntry represents a Kubernetes object entry in the Flux inventory.
type InventoryEntry struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

// inventoryEntryFrom creates an InventoryEntry from the given id and version.
func inventoryEntryFrom(id, v string) (*InventoryEntry, error) {
	objMetadata, err := object.ParseObjMetadata(id)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   objMetadata.GroupKind.Group,
		Kind:    objMetadata.GroupKind.Kind,
		Version: v,
	})
	u.SetName(objMetadata.Name)
	u.SetNamespace(objMetadata.Namespace)

	return &InventoryEntry{
		Name:       u.GetName(),
		Namespace:  u.GetNamespace(),
		Kind:       u.GetKind(),
		APIVersion: u.GetAPIVersion(),
	}, nil
}

// getInventory returns the inventory of Kubernetes object entries that are managed by the Flux.
// In the case of a HelmRelease, it extracts the metadata from the Helm storage secret belonging
// to the latest release version.
func (h *Handler) getInventory(
	ctx context.Context,
	obj unstructured.Unstructured,
) ([]InventoryEntry, error) {
	inv := make([]InventoryEntry, 0)

	// If kind is ArtifactGenerator, extract ExternalArtifacts from status.inventory[]
	if obj.GetKind() == fluxcdv1.FluxArtifactGeneratorKind {
		if artifacts, exists, _ := unstructured.NestedSlice(obj.Object, "status", "inventory"); exists && len(artifacts) > 0 {
			for _, artifact := range artifacts {
				if artifactMap, ok := artifact.(map[string]any); ok {
					name, found := artifactMap["name"].(string)
					if !found {
						continue
					}
					namespace, found := artifactMap["namespace"].(string)
					if !found {
						continue
					}
					inv = append(inv, InventoryEntry{
						Name:       name,
						Namespace:  namespace,
						Kind:       fluxcdv1.FluxExternalArtifactKind,
						APIVersion: fmt.Sprintf("%s/%s", fluxcdv1.FluxSourceGroup, "v1"),
					})
				}
			}
			return inv, nil
		}
	}

	// If the object has a status.inventory.entries field, extract the entries.
	if entries, exists, _ := unstructured.NestedSlice(obj.Object, "status", "inventory", "entries"); exists && len(entries) > 0 {
		for _, entry := range entries {
			if entryMap, ok := entry.(map[string]any); ok {
				id, found := entryMap["id"].(string)
				if !found {
					continue
				}
				v, found := entryMap["v"].(string)
				if !found {
					continue
				}
				if invEntry, err := inventoryEntryFrom(id, v); err == nil {
					inv = append(inv, *invEntry)
				}
			}
		}
		return inv, nil
	}

	// Special handling for HelmRelease to extract inventory from Helm storage
	if obj.GetKind() == fluxcdv1.FluxHelmReleaseKind {
		return h.getHelmReleaseInventory(ctx, obj)
	}

	return inv, nil
}

// getHelmReleaseInventory extracts the inventory from the Helm storage secret
// belonging to the latest release version of a HelmRelease.
// Required for Flux <= 2.7
func (h *Handler) getHelmReleaseInventory(
	ctx context.Context,
	obj unstructured.Unstructured,
) ([]InventoryEntry, error) {
	inv := make([]InventoryEntry, 0)
	kubeClient := h.kubeClient.GetClient(ctx)

	if _, found, _ := unstructured.NestedFieldCopy(obj.Object, "spec", "kubeConfig"); found {
		// Skip release if it targets a remote cluster
		return nil, nil
	}

	storageNamespace, _, _ := unstructured.NestedString(obj.Object, "status", "storageNamespace")
	history, _, _ := unstructured.NestedSlice(obj.Object, "status", "history")
	if storageNamespace == "" || len(history) == 0 {
		// Skip release with no history
		return nil, nil
	}

	// Get the latest release from the history
	entry, ok := history[0].(map[string]any)
	if !ok {
		return nil, nil
	}
	releaseName, _ := entry["name"].(string)
	releaseVersion, _ := entry["version"].(int64)
	releaseNamespace, _ := entry["namespace"].(string)

	storageKey := client.ObjectKey{
		Namespace: storageNamespace,
		Name:      fmt.Sprintf("sh.helm.release.v1.%s.v%v", releaseName, releaseVersion),
	}

	storageSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, storageKey, storageSecret); err != nil {
		// Skip release if it has no storage
		if errors.IsForbidden(err) {
			return nil, err
		}
		return nil, nil
	}

	releaseData, releaseFound := storageSecret.Data["release"]
	if !releaseFound {
		// Skip release if the storage key is missing
		return nil, nil
	}

	rls, err := inventory.DecodeHelmStorage(releaseData)
	if err != nil {
		// Skip release if the storage cannot be decoded
		return nil, nil
	}

	objects, err := ssautil.ReadObjects(strings.NewReader(rls.Manifest))
	if err != nil {
		// Skip release if the objects in storage cannot be read
		return nil, nil
	}

	// Add the object to the inventory list
	for _, o := range objects {
		// Set the namespace on namespaced objects if missing
		if o.GetNamespace() == "" {
			isNamespaced, err := apiutil.IsObjectNamespaced(o, kubeClient.Scheme(), kubeClient.RESTMapper())
			if err != nil && errors.IsForbidden(err) {
				return nil, err
			}
			if isNamespaced {
				o.SetNamespace(releaseNamespace)
			}
		}
		inv = append(inv, InventoryEntry{
			Name:       o.GetName(),
			Namespace:  o.GetNamespace(),
			Kind:       o.GetKind(),
			APIVersion: o.GetAPIVersion(),
		})
	}

	// If the HelmRelease has CRDs to install or upgrade, we need to add them to the inventory
	_, installCRDs, _ := unstructured.NestedBool(obj.Object, "spec", "install", "crds")
	_, upgradeCRDs, _ := unstructured.NestedBool(obj.Object, "spec", "upgrade", "crds")
	if installCRDs || upgradeCRDs {
		selector := client.MatchingLabels{
			"helm.toolkit.fluxcd.io/name":      obj.GetName(),
			"helm.toolkit.fluxcd.io/namespace": obj.GetNamespace(),
		}
		crdKind := "CustomResourceDefinition"
		var list apiextensionsv1.CustomResourceDefinitionList
		if err := kubeClient.List(ctx, &list, selector); err != nil {
			if errors.IsForbidden(err) {
				return nil, err
			}
		} else {
			for _, crd := range list.Items {
				found := false
				for _, obj := range objects {
					if obj.GetName() == crd.GetName() && obj.GetKind() == crdKind {
						found = true
						break
					}
				}

				if !found {
					inv = append(inv, InventoryEntry{
						Name:       crd.GetName(),
						Kind:       crdKind,
						APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
					})
				}
			}
		}
	}

	return inv, nil
}

// preferredFluxGVK returns the preferred GroupVersionKind for a given Flux kind.
func (h *Handler) preferredFluxGVK(ctx context.Context, kind string) (*schema.GroupVersionKind, error) {
	gk, err := fluxcdv1.FluxGroupFor(kind)
	if err != nil {
		return nil, err
	}

	// This is a core operation on the UI backend that is required for getting
	// or listing Flux resources according to their preferred GVK. If the user
	// has access to the get/list the resource, for a better UX we should not
	// error out if the user does not have permission to get the preferred GVK
	// of the resource. In all the calls to this function, the main operation
	// only succeeds if the user has get/list permissions on the resource itself,
	// so there's no point in enforcing RBAC for this "meta" operation. Thus,
	// we use a privileged client here.
	mapping, err := h.kubeClient.GetClient(ctx, kubeclient.WithPrivileges()).RESTMapper().RESTMapping(*gk)
	if err != nil {
		return nil, err
	}

	return &mapping.GroupVersionKind, nil
}
