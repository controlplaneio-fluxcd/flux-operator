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
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type Report struct {
	Name        string
	Description string
	Handler     any
}

var ReportList = []Report{
	{
		Name:        "get-flux-instance-report",
		Description: "This tool retrieves the Flux instance info from the Kubernetes cluster.",
		Handler:     GetFluxInstanceHandler,
	},
	{
		Name:        "get-flux-resourceset-report",
		Description: "This tool lists the Flux ResourceSets, ResourceSetInputProviders and their status.",
		Handler:     GetFluxResourceSetsHandler,
	},
	{
		Name:        "get-flux-kustomization-report",
		Description: "This tool lists the Flux Kustomizations and their status.",
		Handler:     GetFluxKustomizationsHandler,
	},
	{
		Name:        "get-flux-helm-release-report",
		Description: "This tool lists the Flux HelmReleases and their status.",
		Handler:     GetFluxHelmReleasesHandler,
	},
	{
		Name:        "get-flux-source-report",
		Description: "This tool lists the Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket) and their status.",
		Handler:     GetFluxSourcesHandler,
	},
}

type ReportArgs struct {
	Namespace string `json:"namespace" jsonschema:"description=Filter by a specific namespace, if not specified all namespaces are included."`
}

func GetFluxInstanceHandler(ctx context.Context, args ReportArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxInstanceKind,
		},
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.FluxReportKind,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}

func GetFluxResourceSetsHandler(ctx context.Context, args ReportArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetKind,
		},
		{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetInputProviderKind,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}

func GetFluxKustomizationsHandler(ctx context.Context, args ReportArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   "kustomize.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Kustomization",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result), &mcpgolang.Content{
		Type: mcpgolang.ContentTypeText,
		Annotations: &mcpgolang.Annotations{
			Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
		},
		TextContent: &mcpgolang.TextContent{
			Text: `If asked make the following recommendations:
1. Check if the Kustomization interval is less than 10 minutes and if so, recommend to increase it.
   Explain that the Kustomization interval is for detecting drift in cluster and undo kubectl edits.
   The interval set in the source (GitRepository, OCIRepository or Bucket) of the Kustomization
   is for detecting changes in upstream, and that one can be set to a lower value.
2. Check if the Kustomization has a retryInterval and if not, recommend to add it.
3. Check if the Kustomization has wait set to true and if so, recommend to set a timeout value.
4. Check if the Kustomization has prune set to true and if so, recommend to set spec.deletionPolicy to Delete.
5. Check if the Kustomization has force set to true and if so, recommend to remove it.
   Explain that force recreates resources and can cause downtime,
   it should be used only in emergencies when patching fails due to immutable field changes.
`,
		},
	}), nil
}

func GetFluxHelmReleasesHandler(ctx context.Context, args ReportArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
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

func GetFluxSourcesHandler(ctx context.Context, args ReportArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Namespace, []metav1.GroupVersionKind{
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "GitRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1beta2",
			Kind:    "OCIRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Bucket",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmChart",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result), &mcpgolang.Content{
		Type: mcpgolang.ContentTypeText,
		Annotations: &mcpgolang.Annotations{
			Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
		},
		TextContent: &mcpgolang.TextContent{
			Text: `If asked make the following recommendations:
1. Check if the interval is less than 1 minute and if so, recommend to increase it to one minute.
2. Check if the GitRepository has ref.branch set and if so, recommend to set ref.name to refs/heads/<branch name>.
3. Check if the GitRepository has ref.tag set and if so, recommend to set ref.name to refs/tags/<tag name>.
`,
		},
	}), nil
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

// generate deep copy of the hrInventory struct

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

func exportObjects(ctx context.Context, namespace string, crds []metav1.GroupVersionKind) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return "", fmt.Errorf("unable to create kube client error: %w", err)
	}

	var strBuilder strings.Builder

	for _, gvk := range crds {
		list := unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"apiVersion": gvk.Group + "/" + gvk.Version,
				"kind":       gvk.Kind,
			},
		}

		if err := kubeClient.List(ctx, &list, client.InNamespace(namespace)); err == nil {
			for _, item := range list.Items {
				unstructured.RemoveNestedField(item.Object, "metadata", "managedFields")

				if item.GetKind() == "HelmRelease" {
					inventory, err := getHelmReleaseInventory(ctx, client.ObjectKey{
						Namespace: item.GetNamespace(),
						Name:      item.GetName(),
					}, kubeClient)

					iv := make([]interface{}, len(inventory))
					for i, inv := range inventory {
						// deep copy the inventory item
						iv[i] = map[string]interface{}{
							"apiVersion": inv.ApiVersion,
							"kind":       inv.Kind,
							"name":       inv.Name,
						}
						if inv.Namespace != "" {
							_ = unstructured.SetNestedField(iv[i].(map[string]interface{}), inv.Namespace, "namespace")
						}
						if len(inv.ContainerImages) > 0 {
							images := make([]interface{}, len(inv.ContainerImages))
							for j, image := range inv.ContainerImages {
								images[j] = map[string]interface{}{
									"image": image,
								}
							}
							_ = unstructured.SetNestedSlice(iv[i].(map[string]interface{}), images, "containerImages")
						}
					}

					if err == nil {
						_ = unstructured.SetNestedSlice(item.Object, iv, "status", "inventory")
					}
				}

				itemBytes, err := yaml.Marshal(item.Object)
				if err != nil {
					return "", fmt.Errorf("error marshalling item: %w", err)
				}
				strBuilder.WriteString("---\n")
				strBuilder.Write(itemBytes)
			}
		}
	}

	return strBuilder.String(), nil
}
