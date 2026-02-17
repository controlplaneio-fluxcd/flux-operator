// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/entitlement"
)

func TestGetDistributionStatus_Installed(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitrepositories.source.toolkit.fluxcd.io",
			Labels: map[string]string{
				"app.kubernetes.io/version":    "v2.4.0",
				"app.kubernetes.io/managed-by": "flux-operator",
			},
		},
	}

	r := newTestReporter(scheme, crd)
	result := r.getDistributionStatus(ctx)
	g.Expect(result.Status).To(Equal("Installed"))
	g.Expect(result.Version).To(Equal("v2.4.0"))
	g.Expect(result.ManagedBy).To(Equal("flux-operator"))
}

func TestGetDistributionStatus_NotInstalled(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	r := newTestReporter(scheme)
	result := r.getDistributionStatus(ctx)
	g.Expect(result.Status).To(Equal("Not Installed"))
}

func TestGetDistributionStatus_ManagedByBootstrap(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitrepositories.source.toolkit.fluxcd.io",
			Labels: map[string]string{
				"kustomize.toolkit.fluxcd.io/name": "flux-system",
			},
		},
	}

	r := newTestReporter(scheme, crd)
	result := r.getDistributionStatus(ctx)
	g.Expect(result.Status).To(Equal("Installed"))
	g.Expect(result.ManagedBy).To(Equal("flux bootstrap"))
}

func TestGetDistributionStatus_Entitlement(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-operator-entitlement",
			Namespace: "flux-system",
		},
		Data: map[string][]byte{
			entitlement.TokenKey:  []byte("test-token"),
			entitlement.VendorKey: []byte("ControlPlane"),
		},
	}

	r := newTestReporter(scheme, secret)
	result := r.getDistributionStatus(ctx)
	g.Expect(result.Entitlement).To(Equal("Issued by ControlPlane"))
}
