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

func TestResourceSetReconciler_requestsForResourceSetInputProviders(t *testing.T) {
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
			g := NewWithT(t)
			reqs := reconciler.requestsForResourceSetInputProviders(ctx, tt.obj)
			g.Expect(reqs).To(HaveLen(len(tt.expectedRequests)))
			g.Expect(reqs).To(ContainElements(tt.expectedRequests))
		})
	}
}

func TestResourceSetReconciler_requestsForConfigMapsOrSecrets(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create ConfigMap with ResourceSet owner labels.
	err = testEnv.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-with-labels",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.CopyFromAnnotation: "matching/configmap",
			},
			Labels: map[string]string{
				fluxcdv1.OwnerLabelResourceSetName:      "the-value",
				fluxcdv1.OwnerLabelResourceSetNamespace: "doesnt-matter",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create Secret with ResourceSet owner labels.
	err = testEnv.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-with-labels",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.CopyFromAnnotation: "matching/secret",
			},
			Labels: map[string]string{
				fluxcdv1.OwnerLabelResourceSetName:      "the-value",
				fluxcdv1.OwnerLabelResourceSetNamespace: "doesnt-matter",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create ConfigMap without ResourceSet owner labels.
	err = testEnv.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-without-labels",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.CopyFromAnnotation: "mismatching/configmap",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create Secret without ResourceSet owner labels.
	err = testEnv.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-without-labels",
			Namespace: ns.Name,
			Annotations: map[string]string{
				fluxcdv1.CopyFromAnnotation: "mismatching/secret",
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// TypeMetas.
	cmTypeMeta := metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "ConfigMap",
	}
	secretTypeMeta := metav1.TypeMeta{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Secret",
	}

	// Run tests.
	for _, tt := range []struct {
		name             string
		obj              client.Object
		expectedRequests []reconcile.Request
	}{
		{
			name: "not a ConfigMap or Secret",
			obj: &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "not-a-configmap-or-secret",
					Namespace: ns.Name,
				},
			},
			expectedRequests: nil,
		},
		{
			name: "ConfigMap with ResourceSet owner labels",
			obj: &metav1.PartialObjectMetadata{
				TypeMeta: cmTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configmap",
					Namespace: "matching",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: client.ObjectKey{
					Name:      "the-value",
					Namespace: "doesnt-matter",
				},
			}},
		},
		{
			name: "Secret with ResourceSet owner labels",
			obj: &metav1.PartialObjectMetadata{
				TypeMeta: secretTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "matching",
				},
			},
			expectedRequests: []reconcile.Request{{
				NamespacedName: client.ObjectKey{
					Name:      "the-value",
					Namespace: "doesnt-matter",
				},
			}},
		},
		{
			name: "ConfigMap without ResourceSet owner labels",
			obj: &metav1.PartialObjectMetadata{
				TypeMeta: cmTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cm-without-labels",
					Namespace: ns.Name,
				},
			},
			expectedRequests: nil,
		},
		{
			name: "Secret without ResourceSet owner labels",
			obj: &metav1.PartialObjectMetadata{
				TypeMeta: secretTypeMeta,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-without-labels",
					Namespace: ns.Name,
				},
			},
			expectedRequests: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			reqs := reconciler.requestsForConfigMapsOrSecrets(ctx, tt.obj)
			g.Expect(reqs).To(HaveLen(len(tt.expectedRequests)))
			g.Expect(reqs).To(ContainElements(tt.expectedRequests))
		})
	}
}
