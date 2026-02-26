// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestNewProviderKey(t *testing.T) {
	g := NewWithT(t)

	rsip := &fluxcdv1.ResourceSetInputProvider{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.ResourceSetInputProviderKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-provider",
			Namespace: "test-ns",
		},
	}

	key := inputs.NewProviderKey(rsip)
	g.Expect(key.Name).To(Equal("my-provider"))
	g.Expect(key.Namespace).To(Equal("test-ns"))
	g.Expect(key.GVK.Kind).To(Equal(fluxcdv1.ResourceSetInputProviderKind))
	g.Expect(key.GVK.Group).To(Equal(fluxcdv1.GroupVersion.Group))
	g.Expect(key.GVK.Version).To(Equal(fluxcdv1.GroupVersion.Version))
}

func TestNewProviderKey_ResourceSet(t *testing.T) {
	g := NewWithT(t)

	rset := &fluxcdv1.ResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fluxcdv1.GroupVersion.String(),
			Kind:       fluxcdv1.ResourceSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-rset",
			Namespace: "default",
		},
	}

	key := inputs.NewProviderKey(rset)
	g.Expect(key.Name).To(Equal("my-rset"))
	g.Expect(key.Namespace).To(Equal("default"))
	g.Expect(key.GVK.Kind).To(Equal(fluxcdv1.ResourceSetKind))
}
