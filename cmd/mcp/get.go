// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type GetTool struct {
	Name        string
	Description string
	Handler     any
}

var GetToolList = []GetTool{
	{
		Name:        "get-flux-instance-report",
		Description: "This tool retrieves the Flux instance info from the Kubernetes cluster.",
		Handler:     GetFluxInstanceHandler,
	},
	{
		Name:        "get-flux-resourceset-report",
		Description: "This tool lists the Flux ResourceSets, ResourceSetInputProviders and their status and events.",
		Handler:     GetFluxResourceSetsHandler,
	},
	{
		Name:        "get-flux-kustomization-report",
		Description: "This tool lists the Flux Kustomizations and their status and events.",
		Handler:     GetFluxKustomizationsHandler,
	},
	{
		Name:        "get-flux-helmrelease-report",
		Description: "This tool lists the Flux HelmReleases and their status and events.",
		Handler:     GetFluxHelmReleasesHandler,
	},
	{
		Name:        "get-flux-source-report",
		Description: "This tool lists the Flux sources (GitRepository, OCIRepository, HelmRepository, HelmChart, Bucket) and their status and events.",
		Handler:     GetFluxSourcesHandler,
	},
}

type GetArgs struct {
	Name      string `json:"name" jsonschema:"description=Filter by a specific name."`
	Namespace string `json:"namespace" jsonschema:"description=Filter by a specific namespace, if not specified all namespaces are included."`
}

func exportObjects(ctx context.Context, name string, namespace string, crds []metav1.GroupVersionKind) (string, error) {
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

		listOpts := []client.ListOption{
			client.InNamespace(namespace),
		}

		if name != "" {
			listOpts = append(listOpts, client.MatchingFieldsSelector{
				Selector: fields.OneTermEqualSelector("metadata.name", name),
			})
		}

		if err := kubeClient.List(ctx, &list, listOpts...); err == nil {
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

				events, err := getEvents(ctx, kubeClient, item.GetKind(), item.GetName(), item.GetNamespace())
				if err == nil && len(events) > 0 {
					ev := make([]interface{}, len(events))
					for i, event := range events {
						ev[i] = map[string]interface{}{
							"lastTimestamp": event.LastTimestamp.Time.Format(time.RFC3339),
							"type":          event.Type,
							"message":       event.Message,
						}
					}
					_ = unstructured.SetNestedSlice(item.Object, ev, "status", "events")
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
