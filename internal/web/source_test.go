// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGetResource_HelmReleaseSource(t *testing.T) {
	g := NewWithT(t)

	cleanup := installSourceTestCRDs(t)
	defer cleanup()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "source-"}}
	g.Expect(testClient.Create(ctx, ns)).To(Succeed())
	defer testClient.Delete(ctx, ns)

	createSourceTestObject(t, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"name": "podinfo-chart", "namespace": ns.Name},
		"spec":       map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo-ref"},
	})
	createSourceTestObject(t, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "OCIRepository",
		"metadata":   map[string]any{"name": ns.Name + "-podinfo-oci", "namespace": ns.Name},
		"spec":       map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
	})
	createSourceTestObject(t, map[string]any{
		"apiVersion": "source.toolkit.fluxcd.io/v1",
		"kind":       "HelmRepository",
		"metadata":   map[string]any{"name": "charts", "namespace": ns.Name},
		"spec":       map[string]any{"url": "https://charts.example.com"},
	})

	tests := []struct {
		name         string
		hrName       string
		spec         map[string]any
		expectSource bool
		expectedKind string
		expectedName string
		expectedURL  string
	}{
		{
			name:         "chartRef",
			hrName:       "podinfo-ref",
			spec:         map[string]any{"chartRef": map[string]any{"kind": "OCIRepository", "name": "podinfo-chart"}},
			expectSource: true,
			expectedKind: "OCIRepository",
			expectedName: "podinfo-chart",
			expectedURL:  "oci://ghcr.io/stefanprodan/charts/podinfo-ref",
		},
		{
			name:   "OCIRepository chart template",
			hrName: "podinfo-oci",
			spec: map[string]any{"chart": map[string]any{
				"kind": "OCIRepository",
				"spec": map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
			}},
			expectSource: true,
			expectedKind: "OCIRepository",
			expectedName: ns.Name + "-podinfo-oci",
			expectedURL:  "oci://ghcr.io/stefanprodan/charts/podinfo",
		},
		{
			name:   "HelmChart template with sourceRef",
			hrName: "podinfo-tpl",
			spec: map[string]any{"chart": map[string]any{
				"spec": map[string]any{
					"chart":     "podinfo",
					"sourceRef": map[string]any{"kind": "HelmRepository", "name": "charts"},
				},
			}},
			expectSource: true,
			expectedKind: "HelmRepository",
			expectedName: "charts",
			expectedURL:  "https://charts.example.com",
		},
		{
			name:   "OCIRepository chart template without generated object",
			hrName: "podinfo-missing",
			spec: map[string]any{"chart": map[string]any{
				"kind": "OCIRepository",
				"spec": map[string]any{"url": "oci://ghcr.io/stefanprodan/charts/podinfo"},
			}},
			expectSource: false,
		},
	}

	handler := &Handler{
		kubeClient:    kubeClient,
		version:       "v1.0.0",
		statusManager: "test-status-manager",
		namespace:     "flux-system",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			createSourceTestObject(t, map[string]any{
				"apiVersion": "helm.toolkit.fluxcd.io/v2",
				"kind":       "HelmRelease",
				"metadata":   map[string]any{"name": tt.hrName, "namespace": ns.Name},
				"spec":       tt.spec,
			})

			resource, err := handler.GetResource(ctx, "HelmRelease", tt.hrName, ns.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resource).NotTo(BeNil())

			sourceRef, found, err := unstructured.NestedStringMap(resource.Object, "status", "sourceRef")
			g.Expect(err).NotTo(HaveOccurred())
			if !tt.expectSource {
				g.Expect(found).To(BeFalse())
				return
			}
			g.Expect(found).To(BeTrue())
			g.Expect(sourceRef["kind"]).To(Equal(tt.expectedKind))
			g.Expect(sourceRef["name"]).To(Equal(tt.expectedName))
			g.Expect(sourceRef["namespace"]).To(Equal(ns.Name))
			g.Expect(sourceRef["url"]).To(Equal(tt.expectedURL))
		})
	}
}

// installSourceTestCRDs installs minimal HelmRelease, OCIRepository and
// HelmRepository CRDs and returns a function that removes them.
func installSourceTestCRDs(t *testing.T) func() {
	t.Helper()
	g := NewWithT(t)

	defs := []struct {
		group, version, kind, plural string
	}{
		{"helm.toolkit.fluxcd.io", "v2", "HelmRelease", "helmreleases"},
		{"source.toolkit.fluxcd.io", "v1", "OCIRepository", "ocirepositories"},
		{"source.toolkit.fluxcd.io", "v1", "HelmRepository", "helmrepositories"},
	}

	crds := make([]*unstructured.Unstructured, 0, len(defs))
	for _, def := range defs {
		crd := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata":   map[string]any{"name": def.plural + "." + def.group},
			"spec": map[string]any{
				"group": def.group,
				"names": map[string]any{
					"kind":     def.kind,
					"listKind": def.kind + "List",
					"plural":   def.plural,
					"singular": strings.ToLower(def.kind),
				},
				"scope": "Namespaced",
				"versions": []any{map[string]any{
					"name":    def.version,
					"served":  true,
					"storage": true,
					"schema": map[string]any{"openAPIV3Schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"spec":   map[string]any{"type": "object", "x-kubernetes-preserve-unknown-fields": true},
							"status": map[string]any{"type": "object", "x-kubernetes-preserve-unknown-fields": true},
						},
					}},
				}},
			},
		}}
		g.Expect(testClient.Create(ctx, crd)).To(Succeed())
		crds = append(crds, crd)

		// Wait until the CRD is established and the API is discoverable.
		listGVK := schema.GroupVersionKind{Group: def.group, Version: def.version, Kind: def.kind + "List"}
		g.Eventually(func() error {
			list := &unstructured.UnstructuredList{}
			list.SetGroupVersionKind(listGVK)
			return testClient.List(ctx, list)
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	}

	return func() {
		for _, crd := range crds {
			_ = testClient.Delete(context.Background(), crd)
		}
	}
}

// createSourceTestObject creates an unstructured object in the test cluster.
func createSourceTestObject(t *testing.T, obj map[string]any) {
	t.Helper()
	g := NewWithT(t)
	u := &unstructured.Unstructured{Object: obj}
	g.Expect(testClient.Create(ctx, u)).To(Succeed())
}
