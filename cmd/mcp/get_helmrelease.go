// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	mcpgolang "github.com/metoro-io/mcp-golang"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func GetFluxHelmReleasesHandler(ctx context.Context, args GetArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Name, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   "helm.toolkit.fluxcd.io",
			Version: "v2",
			Kind:    "HelmRelease",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(
		mcpgolang.NewTextContent(result),
		&mcpgolang.Content{
			Type: mcpgolang.ContentTypeText,
			Annotations: &mcpgolang.Annotations{
				Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
			},
			TextContent: &mcpgolang.TextContent{
				Text: `If asked about container images, exact the image references as they appear in the
HelmRelease status.inventory.containerImages fields, with all tags preserved as they are, do not remove the ':'' or 'v'' characters, use code blocks to display them.
If asked make the following recommendations:
1. Check if the interval is less than 10 minutes and if so, recommend to increase it.
   Explain that the HelmRelease interval is for detecting drift in cluster.
   The interval set in the source (OCIRepository, HelmRepository) of the HelmRelease
   is for detecting changes in upstream Helm chart, and that one can be set to a lower value.
2. Check if the HelmRelease has releaseName set and if not, recommend to add it.
3. Check if the HelmRelease has targetNamespace set, if so check if storageNamespace is set to the same value.
   If not, recommend to set storageNamespace to the same value as targetNamespace.
4. Check if postRenderers are set, if any of the patches have a namespace set in the target, recommend to remove it.
`,
			},
		},
	), nil
}

type hrStorage struct {
	Name     string `json:"name,omitempty"`
	Manifest string `json:"manifest,omitempty"`
}

type hrHistory struct {
	ChartName string `json:"chartName,omitempty"`
	Version   int64  `json:"version,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type hrInventory struct {
	ApiVersion      string   `json:"apiVersion"`
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace,omitempty"`
	ContainerImages []string `json:"containerImages,omitempty"`
}

func getHelmReleaseInventory(ctx context.Context, objectKey client.ObjectKey, kubeClient client.Client) ([]hrInventory, error) {
	inventory := make([]hrInventory, 0)
	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
		},
	}
	if err := kubeClient.Get(ctx, objectKey, hr); err != nil {
		return nil, err
	}

	// skip release if it targets a remote clusters
	if _, found, _ := unstructured.NestedFieldCopy(hr.Object, "spec", "kubeConfig"); found {
		return nil, nil
	}

	storageNamespace, _, _ := unstructured.NestedString(hr.Object, "status", "storageNamespace")
	history, _, _ := unstructured.NestedSlice(hr.Object, "status", "history")
	if storageNamespace == "" || len(history) == 0 {
		// Skip release if it has no current
		return nil, nil
	}

	// get the latest release from the history
	latest := &hrHistory{}
	latest.ChartName = history[0].(map[string]interface{})["chartName"].(string)
	latest.Version = history[0].(map[string]interface{})["version"].(int64)
	latest.Namespace = history[0].(map[string]interface{})["namespace"].(string)

	storageKey := client.ObjectKey{
		Namespace: storageNamespace,
		Name:      fmt.Sprintf("sh.helm.release.v1.%s.v%v", latest.ChartName, latest.Version),
	}

	storageSecret := &corev1.Secret{}
	if err := kubeClient.Get(ctx, storageKey, storageSecret); err != nil {
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
	var rls hrStorage
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
			if isNamespaced, _ := apiutil.IsObjectNamespaced(obj, kubeClient.Scheme(), kubeClient.RESTMapper()); isNamespaced {
				obj.SetNamespace(latest.Namespace)
			}
		}

		// extract container images from the object
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers"); found {
			for _, container := range containers {
				if image, found, _ := unstructured.NestedString(container.(map[string]interface{}), "image"); found {
					containerImages = append(containerImages, image)
				}
			}
		}

		inventory = append(inventory, hrInventory{
			ApiVersion:      obj.GetAPIVersion(),
			Kind:            obj.GetKind(),
			Name:            obj.GetName(),
			Namespace:       obj.GetNamespace(),
			ContainerImages: containerImages,
		})
	}

	return inventory, nil
}
