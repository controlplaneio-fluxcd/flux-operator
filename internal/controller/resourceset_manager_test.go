// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestResourceSetReconciler_requestsForChangeOf(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create RSET matching app=test label.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-app-test-label",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			InputsFrom: []fluxcdv1.InputProviderReference{{
				Kind: fluxcdv1.ResourceSetInputProviderKind,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test"},
				},
			}},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create RSET matching name=test.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "matching-name-test",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			InputsFrom: []fluxcdv1.InputProviderReference{{
				Kind: fluxcdv1.ResourceSetInputProviderKind,
				Name: "test",
			}},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Run tests.
	for _, tt := range []struct {
		name             string
		obj              client.Object
		expectedRequests []reconcile.Request
	}{
		{
			name: "secret object is not supported yet",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.Name,
					Labels:    map[string]string{"app": "test"},
				},
			},
		},
		{
			name: "rsip from another namespace does not match any rsets",
			obj: &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test2",
					Labels:    map[string]string{"app": "test"},
				},
			},
		},
		{
			name: "matching by name and selector",
			obj: &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.Name,
					Labels:    map[string]string{"app": "test"},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Name:      "matching-name-test",
						Namespace: ns.Name,
					},
				},
				{
					NamespacedName: client.ObjectKey{
						Name:      "matching-app-test-label",
						Namespace: ns.Name,
					},
				},
			},
		},
		{
			name: "matching by name",
			obj: &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns.Name,
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Name:      "matching-name-test",
						Namespace: ns.Name,
					},
				},
			},
		},
		{
			name: "matching by selector",
			obj: &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: ns.Name,
					Labels:    map[string]string{"app": "test"},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Name:      "matching-app-test-label",
						Namespace: ns.Name,
					},
				},
			},
		},
		{
			name: "not matching",
			obj: &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: ns.Name,
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reqs := reconciler.requestsForChangeOf(ctx, tt.obj)
			g.Expect(reqs).To(HaveLen(len(tt.expectedRequests)))
			g.Expect(reqs).To(ContainElements(tt.expectedRequests))
		})
	}
}
