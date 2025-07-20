// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFluxManagedObjectReconciler(t *testing.T) {
	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expected    *FluxManagedObjectReconciler
		expectError bool
	}{
		{
			name: "FluxInstance managed object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ServiceAccount",
					"metadata": map[string]any{
						"name":      "kustomize-controller",
						"namespace": "flux-system",
						"labels": map[string]any{
							"app.kubernetes.io/managed-by":     "flux-operator",
							"fluxcd.controlplane.io/name":      "flux",
							"fluxcd.controlplane.io/namespace": "flux-system",
						},
					},
				},
			},
			expected: &FluxManagedObjectReconciler{
				Kind:      "FluxInstance",
				Name:      "flux",
				Namespace: "flux-system",
			},
			expectError: false,
		},
		{
			name: "Kustomization managed object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "test-cm",
						"namespace": "default",
						"labels": map[string]any{
							"kustomize.toolkit.fluxcd.io/name":      "podinfo",
							"kustomize.toolkit.fluxcd.io/namespace": "default",
						},
					},
				},
			},
			expected: &FluxManagedObjectReconciler{
				Kind:      "Kustomization",
				Name:      "podinfo",
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name: "HelmRelease managed object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
						"labels": map[string]any{
							"helm.toolkit.fluxcd.io/name":      "nginx",
							"helm.toolkit.fluxcd.io/namespace": "nginx-system",
						},
					},
				},
			},
			expected: &FluxManagedObjectReconciler{
				Kind:      "HelmRelease",
				Name:      "nginx",
				Namespace: "nginx-system",
			},
			expectError: false,
		},
		{
			name: "ResourceSet managed object",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"name":      "test-svc",
						"namespace": "default",
						"labels": map[string]any{
							"resourceset.fluxcd.controlplane.io/name":      "my-resources",
							"resourceset.fluxcd.controlplane.io/namespace": "flux-system",
						},
					},
				},
			},
			expected: &FluxManagedObjectReconciler{
				Kind:      "ResourceSet",
				Name:      "my-resources",
				Namespace: "flux-system",
			},
			expectError: false,
		},
		{
			name: "Object not managed by Flux",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "test-cm",
						"namespace": "default",
						"labels": map[string]any{
							"app": "my-app",
						},
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "Object with incomplete Flux labels",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "test-cm",
						"namespace": "default",
						"labels": map[string]any{
							"kustomize.toolkit.fluxcd.io/name": "podinfo",
							// Missing namespace label
						},
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "Object with no labels",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "test-cm",
						"namespace": "default",
					},
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := getFluxReconciler(tt.obj)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(result.Kind).To(Equal(tt.expected.Kind))
			g.Expect(result.Name).To(Equal(tt.expected.Name))
			g.Expect(result.Namespace).To(Equal(tt.expected.Namespace))
		})
	}
}
