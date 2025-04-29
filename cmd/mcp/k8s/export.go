// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// Export retrieves resources based on provided criteria and returns them as a YAML multi-document string.
// Supports filtering by GroupVersionKind, name, namespace, labels, and limit.
// Allows masking secrets and includes additional metadata for Flux resource types.
func (k *Client) Export(ctx context.Context,
	gvks []schema.GroupVersionKind,
	name, namespace, labelSelector string,
	limit int,
	maskSecrets bool) (string, error) {
	var strBuilder strings.Builder
	for _, gvk := range gvks {
		list := unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"apiVersion": gvk.Group + "/" + gvk.Version,
				"kind":       gvk.Kind,
			},
		}

		listOpts := []ctrlclient.ListOption{
			ctrlclient.InNamespace(namespace),
		}

		if limit > 0 {
			listOpts = append(listOpts, ctrlclient.Limit(limit))
		}

		if name != "" {
			listOpts = append(listOpts, ctrlclient.MatchingFieldsSelector{
				Selector: fields.OneTermEqualSelector("metadata.name", name),
			})
		}

		if labelSelector != "" {
			sel, err := labels.Parse(labelSelector)
			if err != nil {
				return "", fmt.Errorf("invalid label selector format: %w", err)
			}

			listOpts = append(listOpts, ctrlclient.MatchingLabelsSelector{Selector: sel})
		}

		if err := k.Client.List(ctx, &list, listOpts...); err == nil {
			for _, item := range list.Items {
				unstructured.RemoveNestedField(item.Object, "metadata", "managedFields")

				if item.GetKind() == "Secret" && maskSecrets {
					dataKV, found, err := unstructured.NestedMap(item.Object, "data")
					if err == nil && found {
						for k := range dataKV {
							_ = unstructured.SetNestedField(item.Object, "****", "data", k)
						}
					}
				}

				if item.GetKind() == "HelmRelease" {
					inventory, err := k.GetHelmInventory(ctx, item.GetAPIVersion(), ctrlclient.ObjectKey{
						Namespace: item.GetNamespace(),
						Name:      item.GetName(),
					})

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
							_ = unstructured.SetNestedSlice(iv[i].(map[string]interface{}), images, "containers")
						}
					}

					if err == nil {
						_ = unstructured.SetNestedSlice(item.Object, iv, "status", "inventory")
					}
				}

				if strings.Contains(item.GetAPIVersion(), "fluxcd") {
					events, err := k.GetEvents(ctx, item.GetKind(), item.GetName(), item.GetNamespace(), "ReconciliationSucceeded")
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

// ExportAPIs retrieves the Kubernetes CRDs and returns the
// preferred API version for each kind as a YAML multi-doc.
func (k *Client) ExportAPIs(ctx context.Context) (string, error) {
	var list apiextensionsv1.CustomResourceDefinitionList
	if err := k.Client.List(ctx, &list, ctrlclient.InNamespace("")); err != nil {
		return "", fmt.Errorf("failed to list CRDs: %w", err)
	}

	if len(list.Items) == 0 {
		return "", errors.New("no CRDs found")
	}

	gvkList := make([]metav1.GroupVersionKind, len(list.Items))
	for i, crd := range list.Items {
		gvk := metav1.GroupVersionKind{
			Group: crd.Spec.Group,
			Kind:  crd.Spec.Names.Kind,
		}
		versions := crd.Status.StoredVersions
		if len(versions) > 0 {
			gvk.Version = versions[len(versions)-1]
		} else {
			return "", fmt.Errorf("no stored versions found for CRD %s", crd.Name)
		}
		gvkList[i] = gvk
	}

	var strBuilder strings.Builder
	for _, gvk := range gvkList {
		itemBytes, err := yaml.Marshal(gvk)
		if err != nil {
			return "", fmt.Errorf("error marshalling gvk: %w", err)
		}
		strBuilder.WriteString("---\n")
		strBuilder.Write(itemBytes)
	}

	return strBuilder.String(), nil
}
