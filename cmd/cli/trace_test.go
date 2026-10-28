// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestTraceCmd_HelmReleaseSource(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	cleanup := installTraceTestCRDs(ctx, g)
	defer cleanup()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "trace-"}}
	g.Expect(testClient.Create(ctx, ns)).To(Succeed())
	defer testClient.Delete(ctx, ns)

	createTraceTestObject(ctx, g, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"name": ns.Name + "-podinfo", "namespace": ns.Name},
		"spec":       map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
	})
	createTraceTestObject(ctx, g, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "HelmRepository",
		"metadata":   map[string]any{"name": "charts", "namespace": ns.Name},
		"spec":       map[string]any{"url": "https://charts.example.com"},
	})

	tests := []struct {
		name        string
		chart       map[string]any
		expectedOut []string
	}{
		{
			name: "OCIRepository chart template",
			chart: map[string]any{
				"kind": "OCIRepository",
				"spec": map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
			},
			expectedOut: []string{
				"Source:          OCIRepository/" + ns.Name + "/" + ns.Name + "-podinfo",
				"Source URL:      oci://ghcr.io/stefanprodan/charts/podinfo",
			},
		},
		{
			name: "HelmChart template with sourceRef",
			chart: map[string]any{
				"spec": map[string]any{
					"chart":     "podinfo",
					"sourceRef": map[string]any{"kind": "HelmRepository", "name": "charts"},
				},
			},
			expectedOut: []string{
				"Source:          HelmRepository/" + ns.Name + "/charts",
				"Source URL:      https://charts.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			hr := createTraceTestObject(ctx, g, map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata":   map[string]any{"name": "podinfo", "namespace": ns.Name},
				"spec":       map[string]any{"chart": tt.chart},
			})
			defer testClient.Delete(ctx, hr)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podinfo",
					Namespace: ns.Name,
					Labels: map[string]string{
						"helm.toolkit.fluxcd.io/name":      "podinfo",
						"helm.toolkit.fluxcd.io/namespace": ns.Name,
					},
				},
			}
			g.Expect(testClient.Create(ctx, cm)).To(Succeed())
			defer testClient.Delete(ctx, cm)

			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand([]string{"trace", "configmap/podinfo"})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(ContainSubstring("Reconciler:      HelmRelease/" + ns.Name + "/podinfo"))
			for _, expected := range tt.expectedOut {
				g.Expect(output).To(ContainSubstring(expected))
			}
		})
	}
}

// installTraceTestCRDs installs minimal HelmRelease, OCIRepository and
// HelmRepository CRDs and returns a function that removes them.
func installTraceTestCRDs(ctx context.Context, g *WithT) func() {
	schemaProps := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"spec":   {Type: "object", XPreserveUnknownFields: new(true)},
			"status": {Type: "object", XPreserveUnknownFields: new(true)},
		},
	}

	defs := []struct {
		group, version, kind, plural string
	}{
		{"helm.toolkit.fluxcd.io", "v2", "HelmRelease", "helmreleases"},
		{"source.toolkit.fluxcd.io", "v1", "OCIRepository", "ocirepositories"},
		{"source.toolkit.fluxcd.io", "v1", "HelmRepository", "helmrepositories"},
	}

	crds := make([]*apiextensionsv1.CustomResourceDefinition, 0, len(defs))
	for _, def := range defs {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: def.plural + "." + def.group},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: def.group,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   def.plural,
					Singular: strings.ToLower(def.kind),
					Kind:     def.kind,
					ListKind: def.kind + "List",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
					Name:    def.version,
					Served:  true,
					Storage: true,
					Schema:  &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: schemaProps},
				}},
			},
		}
		g.Expect(testClient.Create(ctx, crd)).To(Succeed())
		crds = append(crds, crd)

		// Wait until the CRD is established and the API is discoverable.
		listGVK := schema.GroupVersionKind{Group: def.group, Version: def.version, Kind: def.kind + "List"}
		g.Eventually(func() error {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(listGVK)
			return testClient.List(ctx, list)
		}, timeout, "500ms").Should(Succeed())
	}

	return func() {
		for _, crd := range crds {
			_ = testClient.Delete(context.Background(), crd)
		}
	}
}

// createTraceTestObject creates an unstructured object in the test cluster.
func createTraceTestObject(ctx context.Context, g *WithT, obj map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: obj}
	g.Expect(testClient.Create(ctx, u)).To(Succeed())
	return u
}
