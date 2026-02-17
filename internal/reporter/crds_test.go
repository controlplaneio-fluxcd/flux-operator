// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGvkFor(t *testing.T) {
	crds := []metav1.GroupVersionKind{
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
	}

	t.Run("found", func(t *testing.T) {
		g := NewWithT(t)
		gvk := gvkFor("GitRepository", crds)
		g.Expect(gvk).ToNot(BeNil())
		g.Expect(gvk.Group).To(Equal("source.toolkit.fluxcd.io"))
		g.Expect(gvk.Version).To(Equal("v1"))
		g.Expect(gvk.Kind).To(Equal("GitRepository"))
	})

	t.Run("not found", func(t *testing.T) {
		g := NewWithT(t)
		gvk := gvkFor("HelmRelease", crds)
		g.Expect(gvk).To(BeNil())
	})
}

func TestListCRDs_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	crd1 := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitrepositories.source.toolkit.fluxcd.io",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "source.toolkit.fluxcd.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: "GitRepository",
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1"},
		},
	}

	crd2 := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kustomizations.kustomize.toolkit.fluxcd.io",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kustomize.toolkit.fluxcd.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: "Kustomization",
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			StoredVersions: []string{"v1beta1", "v1"},
		},
	}

	r := newTestReporter(scheme, crd1, crd2)
	gvks, err := r.listCRDs(ctx)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(gvks).To(HaveLen(2))

	// Verify the last stored version is used.
	for _, gvk := range gvks {
		g.Expect(gvk.Version).To(Equal("v1"))
	}
}

func TestListCRDs_NoCRDs(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	r := newTestReporter(scheme)
	_, err := r.listCRDs(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no Flux CRDs found"))
}

func TestListCRDs_NoStoredVersions(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(Succeed())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitrepositories.source.toolkit.fluxcd.io",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "flux",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "source.toolkit.fluxcd.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: "GitRepository",
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			StoredVersions: []string{},
		},
	}

	r := newTestReporter(scheme, crd)
	_, err := r.listCRDs(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no stored versions found"))
}
