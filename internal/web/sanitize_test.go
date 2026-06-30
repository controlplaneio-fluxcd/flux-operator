// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCleanObjectForExport(t *testing.T) {
	for _, tt := range []struct {
		name       string
		input      map[string]any
		keepStatus bool
		expected   map[string]any
	}{
		{
			name: "removes status when keepStatus is false",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "keeps status when keepStatus is true",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
			keepStatus: true,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"status": map[string]any{
					"phase": "Active",
				},
			},
		},
		{
			name: "removes runtime metadata fields",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":              "test",
					"namespace":         "default",
					"uid":               "12345",
					"resourceVersion":   "67890",
					"generation":        int64(1),
					"creationTimestamp": "2025-01-01T00:00:00Z",
					"managedFields":     []any{},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "preserves labels and annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
						"env": "prod",
					},
					"annotations": map[string]any{
						"description": "test config",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
						"env": "prod",
					},
					"annotations": map[string]any{
						"description": "test config",
					},
				},
			},
		},
		{
			name: "removes Flux ownership labels",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                                   "myapp",
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"helm.toolkit.fluxcd.io/name":           "my-release",
						"helm.toolkit.fluxcd.io/namespace":      "default",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app": "myapp",
					},
				},
			},
		},
		{
			name: "removes kubectl and Flux CLI annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"description": "keep this",
						"kubectl.kubernetes.io/last-applied-configuration": "{}",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
						"reconcile.fluxcd.io/forceAt":                      "2025-01-01T00:00:00Z",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"description": "keep this",
					},
				},
			},
		},
		{
			name: "removes empty labels map after cleanup",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "removes empty annotations map after cleanup",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"annotations": map[string]any{
						"kubectl.kubernetes.io/last-applied-configuration": "{}",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "handles object without namespace",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "test",
					"uid":  "12345",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "test",
				},
			},
		},
		{
			name: "handles object without labels and annotations",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
			},
		},
		{
			name: "keeps non-Flux ownership labels",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                                   "myapp",
						"kustomize.toolkit.fluxcd.io/name":      "flux-system",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"kustomize.toolkit.fluxcd.io/prune":     "disabled",
					},
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
					"labels": map[string]any{
						"app":                               "myapp",
						"kustomize.toolkit.fluxcd.io/prune": "disabled",
					},
				},
			},
		},
		{
			name: "masks Secret data while cleaning metadata",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":          "creds",
					"namespace":     "default",
					"uid":           "12345",
					"managedFields": []any{},
				},
				"type": "Opaque",
				"data": map[string]any{
					"token": "c2VjcmV0",
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "creds",
					"namespace": "default",
				},
				"type": "Opaque",
				"data": map[string]any{
					"token": "***redacted***",
				},
			},
		},
		{
			name: "complex real-world example",
			input: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":              "my-app",
					"namespace":         "production",
					"uid":               "abc-123",
					"resourceVersion":   "12345",
					"generation":        int64(5),
					"creationTimestamp": "2025-01-01T00:00:00Z",
					"labels": map[string]any{
						"app":                                   "my-app",
						"version":                               "v1.0.0",
						"kustomize.toolkit.fluxcd.io/name":      "apps",
						"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
						"helm.toolkit.fluxcd.io/name":           "my-chart",
						"helm.toolkit.fluxcd.io/namespace":      "flux-system",
					},
					"annotations": map[string]any{
						"description": "My application",
						"kubectl.kubernetes.io/last-applied-configuration": "large-json-blob",
						"reconcile.fluxcd.io/requestedAt":                  "2025-01-01T00:00:00Z",
						"reconcile.fluxcd.io/forceAt":                      "2025-01-01T01:00:00Z",
						"custom.io/annotation":                             "keep-this",
					},
					"managedFields": []any{},
				},
				"spec": map[string]any{
					"replicas": int64(3),
				},
				"status": map[string]any{
					"availableReplicas": int64(3),
				},
			},
			keepStatus: false,
			expected: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]any{
					"name":      "my-app",
					"namespace": "production",
					"labels": map[string]any{
						"app":     "my-app",
						"version": "v1.0.0",
					},
					"annotations": map[string]any{
						"description":          "My application",
						"custom.io/annotation": "keep-this",
					},
				},
				"spec": map[string]any{
					"replicas": int64(3),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create unstructured object from input
			obj := &unstructured.Unstructured{Object: tt.input}

			// Call the function
			cleanObjectForExport(obj, tt.keepStatus)

			// Verify the result matches expected
			g.Expect(obj.Object).To(Equal(tt.expected))
		})
	}
}

func TestMaskSecret(t *testing.T) {
	for _, tt := range []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "masks secret data values keeping keys",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"type": "Opaque",
				"data": map[string]any{
					"username": "YWRtaW4=",
					"password": "c3VwZXJzZWNyZXQ=",
				},
			},
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"type": "Opaque",
				"data": map[string]any{
					"username": "***redacted***",
					"password": "***redacted***",
				},
			},
		},
		{
			name: "masks non-string data values regardless of type",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{
					"username": "YWRtaW4=",
					"weird":    int64(42),
				},
			},
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{
					"username": "***redacted***",
					"weird":    "***redacted***",
				},
			},
		},
		{
			name: "is a no-op for non-Secret kinds",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{
					"key": "value",
				},
			},
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{
					"key": "value",
				},
			},
		},
		{
			name: "handles Secret without data",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"type": "Opaque",
			},
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"type": "Opaque",
			},
		},
		{
			name: "handles Secret with empty data",
			input: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{},
			},
			expected: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test",
					"namespace": "default",
				},
				"data": map[string]any{},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			obj := &unstructured.Unstructured{Object: tt.input}

			maskSecretValues(obj)

			g.Expect(obj.Object).To(Equal(tt.expected))
		})
	}
}
