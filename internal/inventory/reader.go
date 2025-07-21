// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inventory

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// FromStatusOf inspects the status of a Kubernetes object and extracts the
// inventory entries from it. This function is suitable for the following kinds:
// Flux Kustomization, ResourceSet, and FluxInstance.
func FromStatusOf(
	ctx context.Context,
	kubeClient client.Client,
	ref fluxcdv1.ResourceRef,
) ([]fluxcdv1.ResourceRef, error) {
	result := make([]fluxcdv1.ResourceRef, 0)

	obj, err := EntryToUnstructured(ref)
	if err != nil {
		return nil, err
	}

	objKey := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	// Get the object whose status we want to inspect.
	if err := kubeClient.Get(ctx, objKey, obj); err != nil {
		return nil, fmt.Errorf("failed to get %s/%s: %w", obj.GetKind(), objKey.String(), err)
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
				result = append(result, fluxcdv1.ResourceRef{ID: id, Version: v})
			}
		}
	}

	return result, nil
}

// HelmStorage is a struct used to decode the Helm storage secret.
type HelmStorage struct {
	Name     string `json:"name,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

// HelmHistory is a struct used to decode the release
// history from the HelmRelease status.
type HelmHistory struct {
	ChartName string `json:"chartName,omitempty"`
	Version   int64  `json:"version,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// FromHelmRelease returns the inventory of Kubernetes object refs
// that are managed by the HelmRelease. It extracts the metadata from the
// Helm storage secret belonging to the latest release version.
func FromHelmRelease(
	ctx context.Context,
	kubeClient client.Client,
	hr fluxcdv1.ResourceRef,
) ([]fluxcdv1.ResourceRef, error) {
	result := make([]fluxcdv1.ResourceRef, 0)

	hrObj, err := EntryToUnstructured(hr)
	if err != nil {
		return nil, err
	}

	hrKey := client.ObjectKey{
		Namespace: hrObj.GetNamespace(),
		Name:      hrObj.GetName(),
	}

	// Get the HelmRelease object
	if err := kubeClient.Get(ctx, hrKey, hrObj); err != nil {
		return nil, fmt.Errorf("failed to get HelmRelease/%s: %w", hrKey.String(), err)
	}

	if _, found, _ := unstructured.NestedFieldCopy(hrObj.Object, "spec", "kubeConfig"); found {
		// Skip release if it targets a remote cluster
		return nil, nil
	}

	storageNamespace, _, _ := unstructured.NestedString(hrObj.Object, "status", "storageNamespace")
	history, _, _ := unstructured.NestedSlice(hrObj.Object, "status", "history")
	if storageNamespace == "" || len(history) == 0 {
		// Skip release with no history
		return nil, nil
	}

	// Get the latest release from the history
	latest := &HelmHistory{}
	latest.ChartName = history[0].(map[string]any)["chartName"].(string)
	latest.Version = history[0].(map[string]any)["version"].(int64)
	latest.Namespace = history[0].(map[string]any)["namespace"].(string)

	storageKey := client.ObjectKey{
		Namespace: storageNamespace,
		Name:      fmt.Sprintf("sh.helm.release.v1.%s.v%v", latest.ChartName, latest.Version),
	}

	storageSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, storageKey, storageSecret); err != nil {
		// Skip release if it has no storage
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage secret not found for HelmRelease/%s: %w", hrKey.String(), err)
	}

	releaseData, releaseFound := storageSecret.Data["release"]
	if !releaseFound {
		return nil, fmt.Errorf("storage not found for HelmRelease/%s", hrKey.String())
	}

	rls, err := decodeHelmStorage(releaseData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode storage for HelmRelease/%s: %w", hrKey.String(), err)
	}

	objects, err := ssautil.ReadObjects(strings.NewReader(rls.Manifest))
	if err != nil {
		return nil, fmt.Errorf("failed to read storage for HelmRelease/%s: %w", hrKey.String(), err)
	}

	// Add the object to the inventory list
	for _, obj := range objects {
		// Set the namespace on namespaced objects if missing
		if obj.GetNamespace() == "" {
			if isNamespaced, _ := apiutil.IsObjectNamespaced(obj,
				kubeClient.Scheme(), kubeClient.RESTMapper()); isNamespaced {
				obj.SetNamespace(latest.Namespace)
			}
		}
		objMetadata := object.UnstructuredToObjMetadata(obj)
		result = append(result, fluxcdv1.ResourceRef{
			ID:      objMetadata.String(),
			Version: obj.GetObjectKind().GroupVersionKind().Version,
		})
	}

	// If the HelmRelease has CRDs to upgrade, we need to add them to the inventory
	if _, found, _ := unstructured.NestedFieldCopy(hrObj.Object, "spec", "upgrade", "crds"); found {
		selector := client.MatchingLabels{
			"helm.toolkit.fluxcd.io/name":      hrObj.GetName(),
			"helm.toolkit.fluxcd.io/namespace": hrObj.GetNamespace(),
		}
		crdKind := "CustomResourceDefinition"
		var list apiextensionsv1.CustomResourceDefinitionList
		if err := kubeClient.List(ctx, &list, selector); err == nil {
			for _, crd := range list.Items {
				found := false
				for _, obj := range objects {
					if obj.GetName() == crd.GetName() && obj.GetKind() == crdKind {
						found = true
						break
					}
				}

				if !found {
					mo := object.ObjMetadata{
						Name: crd.GetName(),
						GroupKind: schema.GroupKind{
							Group: apiextensionsv1.GroupName,
							Kind:  crdKind,
						},
					}
					result = append(result, fluxcdv1.ResourceRef{
						ID:      mo.String(),
						Version: "v1",
					})
				}
			}
		}
	}
	return result, nil
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
