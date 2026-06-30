// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// cleanObjectForExport sanitizes an object for display: it masks Secret data,
// strips runtime metadata (keeping only name, namespace, labels and annotations),
// drops Flux ownership labels and kubectl/Flux annotations, and removes the
// status subresource unless keepStatus is set.
func cleanObjectForExport(obj *unstructured.Unstructured, keepStatus bool) {
	// Never expose Secret values (no-op for other kinds).
	maskSecretValues(obj)

	// Remove status subresource
	if !keepStatus {
		unstructured.RemoveNestedField(obj.Object, "status")
	}

	// Remove runtime metadata - keep only name, namespace, labels, and annotations.
	// A Get result always carries a metadata map, but guard against a malformed
	// object so a bad type assertion cannot panic the caller.
	metadata, ok := obj.Object["metadata"].(map[string]any)
	if !ok {
		return
	}
	cleanMetadata := make(map[string]any)

	// Preserve essential fields
	if name, exists := metadata["name"]; exists {
		cleanMetadata["name"] = name
	}
	if namespace, exists := metadata["namespace"]; exists {
		cleanMetadata["namespace"] = namespace
	}
	if lb, exists := metadata["labels"]; exists {
		cleanMetadata["labels"] = lb
	}

	if annotations, exists := metadata["annotations"]; exists {
		cleanMetadata["annotations"] = annotations
	}

	// Remove Flux ownership labels
	if lb, exists := cleanMetadata["labels"]; exists {
		if labelMap, ok := lb.(map[string]any); ok {
			for key := range labelMap {
				if fluxcdv1.IsFluxAPI(key) &&
					(strings.HasSuffix(key, "/name") || strings.HasSuffix(key, "/namespace")) {
					delete(labelMap, key)
				}
			}
			// Remove labels map if empty after cleanup
			if len(labelMap) == 0 {
				delete(cleanMetadata, "labels")
			}
		}
	}

	// Remove kubectl and Flux CLI annotations from clean metadata
	if annotations, exists := cleanMetadata["annotations"]; exists {
		if annotationMap, ok := annotations.(map[string]any); ok {
			delete(annotationMap, "kubectl.kubernetes.io/last-applied-configuration")
			delete(annotationMap, "reconcile.fluxcd.io/requestedAt")
			delete(annotationMap, "reconcile.fluxcd.io/forceAt")
			// Remove annotations map if empty after cleanup
			if len(annotationMap) == 0 {
				delete(cleanMetadata, "annotations")
			}
		}
	}

	// Replace metadata with the clean version
	obj.Object["metadata"] = cleanMetadata
}

// maskSecretValues redacts the values of a Kubernetes Secret's data fields in place,
// keeping the keys. It is a no-op for non-Secret objects.
func maskSecretValues(obj *unstructured.Unstructured) {
	if obj.GetKind() != "Secret" {
		return
	}

	data, found, err := unstructured.NestedFieldNoCopy(obj.Object, "data")
	if err != nil || !found {
		return
	}

	if dataMap, ok := data.(map[string]any); ok {
		for k := range dataMap {
			dataMap[k] = "***redacted***"
		}
	}
}
