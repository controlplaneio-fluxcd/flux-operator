// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

// HelmStorage is a struct used to decode the Helm storage secret.
type HelmStorage struct {
	Name     string `json:"name,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

// HelmHistory is a struct used to decode the release
// history from the HelmRelease status.
type HelmHistory struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Version     int64  `json:"version,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// getInventory returns the inventory of Kubernetes object entries that are managed by the Flux.
// In the case of a HelmRelease, it extracts the metadata from the Helm storage secret belonging
// to the latest release version.
// nolint: gocyclo
func (r *Router) getInventory(
	ctx context.Context,
	obj unstructured.Unstructured,
) ([]InventoryEntry, error) {
	inventory := make([]InventoryEntry, 0)
	kubeClient := r.kubeClient.GetClient(ctx)

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
					inventory = append(inventory, InventoryEntry{
						Name:       name,
						Namespace:  namespace,
						Kind:       fluxcdv1.FluxExternalArtifactKind,
						APIVersion: fmt.Sprintf("%s/%s", fluxcdv1.FluxSourceGroup, "v1"),
					})
				}
			}
			return inventory, nil
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
					inventory = append(inventory, *invEntry)
				}
			}
		}
		return inventory, nil
	}

	// Special handling for HelmRelease to extract inventory from Helm storage
	if obj.GetKind() == fluxcdv1.FluxHelmReleaseKind {
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
		latest := &HelmHistory{}
		latest.ReleaseName = history[0].(map[string]any)["name"].(string)
		latest.Version = history[0].(map[string]any)["version"].(int64)
		latest.Namespace = history[0].(map[string]any)["namespace"].(string)

		storageKey := client.ObjectKey{
			Namespace: storageNamespace,
			Name:      fmt.Sprintf("sh.helm.release.v1.%s.v%v", latest.ReleaseName, latest.Version),
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

		rls, err := decodeHelmStorage(releaseData)
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
					obj.SetNamespace(latest.Namespace)
				}
			}
			inventory = append(inventory, InventoryEntry{
				Name:       o.GetName(),
				Namespace:  o.GetNamespace(),
				Kind:       o.GetKind(),
				APIVersion: o.GetAPIVersion(),
			})
		}

		// If the HelmRelease has CRDs to upgrade, we need to add them to the inventory
		if _, found, _ := unstructured.NestedFieldCopy(obj.Object, "spec", "upgrade", "crds"); found {
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
						inventory = append(inventory, InventoryEntry{
							Name:       crd.GetName(),
							Kind:       crdKind,
							APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
						})
					}
				}
			}
		}
	}

	return inventory, nil
}

// decodeHelmStorage decodes the Helm storage secret data into a HelmStorage struct.
// Adapted from https://github.com/helm/helm/blob/02685e94bd3862afcb44f6cd7716dbeb69743567/pkg/storage/driver/util.go
func decodeHelmStorage(releaseData []byte) (*HelmStorage, error) {
	var b64 = base64.StdEncoding
	b, err := b64.DecodeString(string(releaseData))
	if err != nil {
		return nil, err
	}
	var magicGzip = []byte{0x1f, 0x8b, 0x08}
	if bytes.Equal(b[0:3], magicGzip) {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		b2, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		b = b2
	}

	var rls HelmStorage
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, err
	}

	return &rls, nil
}

// preferredFluxGVK returns the preferred GroupVersionKind for a given Flux kind.
func (r *Router) preferredFluxGVK(ctx context.Context, kind string) (*schema.GroupVersionKind, error) {
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
	mapping, err := r.kubeClient.GetClient(ctx, kubeclient.WithPrivileges()).RESTMapper().RESTMapping(*gk)
	if err != nil {
		return nil, err
	}

	return &mapping.GroupVersionKind, nil
}
