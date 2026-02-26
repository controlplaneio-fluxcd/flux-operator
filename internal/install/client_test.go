// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestNewScheme_RegistersExpectedTypes(t *testing.T) {
	g := NewWithT(t)

	s := NewScheme()

	// Verify core types
	g.Expect(s.IsGroupRegistered(corev1.SchemeGroupVersion.Group)).To(BeTrue())

	// Verify apps types
	g.Expect(s.IsGroupRegistered(appsv1.SchemeGroupVersion.Group)).To(BeTrue())

	// Verify RBAC types
	g.Expect(s.IsGroupRegistered(rbacv1.SchemeGroupVersion.Group)).To(BeTrue())

	// Verify apiextensions types
	g.Expect(s.IsGroupRegistered(apiextensionsv1.SchemeGroupVersion.Group)).To(BeTrue())

	// Verify flux-operator types
	g.Expect(s.IsGroupRegistered(fluxcdv1.GroupVersion.Group)).To(BeTrue())
}

func TestNewScheme_FluxInstanceKind(t *testing.T) {
	g := NewWithT(t)

	s := NewScheme()

	// Verify FluxInstance kind can be resolved
	gvk := schema.GroupVersionKind{
		Group:   fluxcdv1.GroupVersion.Group,
		Version: fluxcdv1.GroupVersion.Version,
		Kind:    fluxcdv1.FluxInstanceKind,
	}
	obj, err := s.New(gvk)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obj).NotTo(BeNil())
}
