// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package client

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

	kubeClient := KubeClient{
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

			err = kubeClient.AnnotateResource(
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

	kubeClient := KubeClient{
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

			err = kubeClient.DeleteResource(
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
