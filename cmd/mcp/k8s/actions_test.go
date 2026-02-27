// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestAnnotateResource(t *testing.T) {
	mockNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}

	mockInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/name": "flux",
			},
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithObjects(mockNamespace, mockInstance).
			Build(),
	}

	tests := []struct {
		testName string
		value    string
		matchErr string

		apiVersion string
		kind       string
		name       string
		namespace  string
	}{
		{
			testName: "add annotation",
			value:    "test-annotation",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			name:       "flux",
			namespace:  "flux-system",
		},
		{
			testName: "fails without name",
			value:    "test-annotation",
			matchErr: "not found",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			namespace:  "flux-system",
		},
		{
			testName: "fails without namespace",
			value:    "test-annotation",
			matchErr: "not found",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			name:       "flux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := kubeClient.ParseGroupVersionKind(tt.apiVersion, tt.kind)
			g.Expect(err).NotTo(HaveOccurred())

			err = kubeClient.Annotate(
				context.Background(),
				gvk,
				tt.name,
				tt.namespace,
				[]string{fluxcdv1.ReconcileAnnotation},
				tt.value,
			)

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				result, rErr := kubeClient.Export(
					context.Background(),
					[]schema.GroupVersionKind{gvk},
					"",
					"",
					"app.kubernetes.io/name=flux",
					0,
					false,
				)
				g.Expect(rErr).NotTo(HaveOccurred())
				g.Expect(result).To(ContainSubstring(tt.value))
			}

		})
	}
}

func TestDeleteResource(t *testing.T) {
	mockNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}

	mockInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/name": "flux",
			},
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithObjects(mockNamespace, mockInstance).
			Build(),
	}

	tests := []struct {
		testName string
		matchErr string

		apiVersion string
		kind       string
		name       string
		namespace  string
	}{
		{
			testName: "fails without name",
			matchErr: "not found",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			namespace:  "flux-system",
		},
		{
			testName: "fails without namespace",
			matchErr: "not found",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			name:       "flux",
		},
		{
			testName: "deletes found resource",

			apiVersion: "fluxcd.controlplane.io/v1",
			kind:       "FluxInstance",
			name:       "flux",
			namespace:  "flux-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			gvk, err := kubeClient.ParseGroupVersionKind(tt.apiVersion, tt.kind)
			g.Expect(err).NotTo(HaveOccurred())

			err = kubeClient.Delete(
				context.Background(),
				gvk,
				tt.name,
				tt.namespace,
			)

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))

				result, rErr := kubeClient.Export(
					context.Background(),
					[]schema.GroupVersionKind{gvk},
					"",
					"",
					"app.kubernetes.io/name=flux",
					0,
					false,
				)
				g.Expect(rErr).NotTo(HaveOccurred())
				g.Expect(result).NotTo(BeEmpty())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				result, rErr := kubeClient.Export(
					context.Background(),
					[]schema.GroupVersionKind{gvk},
					"",
					"",
					"app.kubernetes.io/name=flux",
					0,
					false,
				)
				g.Expect(rErr).NotTo(HaveOccurred())
				g.Expect(result).To(BeEmpty())
			}
		})
	}
}

func TestIsManagedByFlux(t *testing.T) {
	mockNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}

	mockManagedInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed",
			Namespace: "flux-system",
			Labels: map[string]string{
				"kustomize.toolkit.fluxcd.io/namespace": "flux-system",
			},
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}

	mockUnmanagedInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unmanaged",
			Namespace: "flux-system",
			Labels: map[string]string{
				"app.kubernetes.io/name": "flux",
			},
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}

	kubeClient := Client{
		Client: fake.NewClientBuilder().
			WithScheme(NewTestScheme()).
			WithObjects(mockNamespace, mockManagedInstance, mockUnmanagedInstance).
			Build(),
	}

	tests := []struct {
		testName  string
		name      string
		namespace string
		managed   bool
	}{
		{
			testName:  "resource managed by Flux",
			name:      "managed",
			namespace: "flux-system",
			managed:   true,
		},
		{
			testName:  "resource not managed by Flux",
			name:      "unmanaged",
			namespace: "flux-system",
			managed:   false,
		},
		{
			testName:  "resource not found",
			name:      "non-existent",
			namespace: "flux-system",
			managed:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			gvk := schema.GroupVersionKind{
				Group:   "fluxcd.controlplane.io",
				Version: "v1",
				Kind:    "FluxInstance",
			}

			result := kubeClient.IsManagedByFlux(
				context.Background(),
				gvk,
				tt.name,
				tt.namespace,
			)
			g.Expect(result).To(Equal(tt.managed))
		})
	}
}

func TestToggleSuspension(t *testing.T) {
	mockNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flux-system",
		},
	}

	mockInstance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "flux",
			Namespace:  "flux-system",
			Generation: 1,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}

	tests := []struct {
		testName string
		suspend  bool
		matchErr string

		name      string
		namespace string
	}{
		{
			testName:  "suspend Flux resource",
			suspend:   true,
			name:      "flux",
			namespace: "flux-system",
		},
		{
			testName:  "resume Flux resource",
			suspend:   false,
			name:      "flux",
			namespace: "flux-system",
		},
		{
			testName:  "fails with non-existent resource",
			suspend:   true,
			name:      "non-existent",
			namespace: "flux-system",
			matchErr:  "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			g := NewWithT(t)

			kubeClient := Client{
				Client: fake.NewClientBuilder().
					WithScheme(NewTestScheme()).
					WithObjects(mockNamespace, mockInstance.DeepCopy()).
					Build(),
			}

			gvk := schema.GroupVersionKind{
				Group:   "fluxcd.controlplane.io",
				Version: "v1",
				Kind:    "FluxInstance",
			}

			err := kubeClient.ToggleSuspension(
				context.Background(),
				gvk,
				tt.name,
				tt.namespace,
				tt.suspend,
			)

			if tt.matchErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
