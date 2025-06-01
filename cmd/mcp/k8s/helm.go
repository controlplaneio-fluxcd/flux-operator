// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

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

// HelmInventory is a struct used to store the metadata of
// the Kubernetes objects that are managed by a HelmRelease.
type HelmInventory struct {
	APIVersion      string   `json:"apiVersion"`
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace,omitempty"`
	ContainerImages []string `json:"containerImages,omitempty"`
}

// GetHelmInventory returns the HelmRelease inventory by extracting the Kubernetes
// objects metadata from the Helm storage secret belonging to the latest release version.
func (k *Client) GetHelmInventory(ctx context.Context, apiVersion string, objectKey ctrlclient.ObjectKey) ([]HelmInventory, error) {
	inventory := make([]HelmInventory, 0)
	hr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       "HelmRelease",
		},
	}
	if err := k.Client.Get(ctx, objectKey, hr); err != nil {
		return nil, err
	}

	// skip release if it targets a remote cluster
	if _, found, _ := unstructured.NestedFieldCopy(hr.Object, "spec", "kubeConfig"); found {
		return nil, nil
	}

	storageNamespace, _, _ := unstructured.NestedString(hr.Object, "status", "storageNamespace")
	history, _, _ := unstructured.NestedSlice(hr.Object, "status", "history")
	if storageNamespace == "" || len(history) == 0 {
		// Skip the release if it has no current
		return nil, nil
	}

	// get the latest release from the history
	latest := &HelmHistory{}
	latest.ChartName = history[0].(map[string]any)["chartName"].(string)
	latest.Version = history[0].(map[string]any)["version"].(int64)
	latest.Namespace = history[0].(map[string]any)["namespace"].(string)

	storageKey := ctrlclient.ObjectKey{
		Namespace: storageNamespace,
		Name:      fmt.Sprintf("sh.helm.release.v1.%s.v%v", latest.ChartName, latest.Version),
	}

	storageSecret := &corev1.Secret{}
	if err := k.Client.Get(ctx, storageKey, storageSecret); err != nil {
		// skip release if it has no storage
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find the Helm storage object for HelmRelease '%s': %w", objectKey.String(), err)
	}

	releaseData, releaseFound := storageSecret.Data["release"]
	if !releaseFound {
		return nil, fmt.Errorf("failed to decode the Helm storage object for HelmRelease '%s'", objectKey.String())
	}

	// adapted from https://github.com/helm/helm/blob/02685e94bd3862afcb44f6cd7716dbeb69743567/pkg/storage/driver/util.go
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

	// extract objects from Helm storage
	var rls HelmStorage
	if err := json.Unmarshal(b, &rls); err != nil {
		return nil, fmt.Errorf("failed to decode the Helm storage object for HelmRelease '%s': %w", objectKey.String(), err)
	}

	objects, err := ssautil.ReadObjects(strings.NewReader(rls.Manifest))
	if err != nil {
		return nil, fmt.Errorf("failed to read the Helm storage object for HelmRelease '%s': %w", objectKey.String(), err)
	}

	containerImages := make([]string, 0)

	// set the namespace on namespaced objects
	for _, obj := range objects {
		if obj.GetNamespace() == "" {
			if isNamespaced, _ := apiutil.IsObjectNamespaced(obj, k.Client.Scheme(), k.Client.RESTMapper()); isNamespaced {
				obj.SetNamespace(latest.Namespace)
			}
		}

		// extract container images from Deployment, StatefulSet, DaemonSet and Job
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers"); found {
			for _, container := range containers {
				if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
					containerImages = append(containerImages, image)
				}
			}
		}

		// extract init container images from Deployment, StatefulSet, DaemonSet and Job
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "initContainers"); found {
			for _, container := range containers {
				if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
					if !slices.Contains(containerImages, image) {
						containerImages = append(containerImages, image)
					}
				}
			}
		}

		// extract container images from CronJob
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "jobTemplate", "spec", "template", "spec", "containers"); found {
			for _, container := range containers {
				if image, found, _ := unstructured.NestedString(container.(map[string]any), "image"); found {
					if !slices.Contains(containerImages, image) {
						containerImages = append(containerImages, image)
					}
				}
			}
		}

		inventory = append(inventory, HelmInventory{
			APIVersion:      obj.GetAPIVersion(),
			Kind:            obj.GetKind(),
			Name:            obj.GetName(),
			Namespace:       obj.GetNamespace(),
			ContainerImages: containerImages,
		})
	}

	return inventory, nil
}
