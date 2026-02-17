// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestGetSyncNameFromInstance_Default(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	r := newTestReporter(scheme)
	name := r.getSyncNameFromInstance(ctx)
	g.Expect(name).To(Equal("flux-system"))
}

func TestGetSyncNameFromInstance_Custom(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "v2.4.0",
				Registry: "ghcr.io/fluxcd",
			},
			Sync: &fluxcdv1.Sync{
				Kind: "GitRepository",
				Name: "my-cluster",
			},
		},
	}

	r := newTestReporter(scheme, instance)
	name := r.getSyncNameFromInstance(ctx)
	g.Expect(name).To(Equal("my-cluster"))
}

func TestGetSyncStatus_NotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	crds := []metav1.GroupVersionKind{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
	}

	r := newTestReporter(scheme)
	syncStatus, err := r.getSyncStatus(ctx, crds)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(syncStatus).To(BeNil())
}

func TestGetSyncStatus_Ready(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	crds := []metav1.GroupVersionKind{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"},
	}

	ks := makeObj("Kustomization", "flux-system", "flux-system")
	setNestedString(ks, "kustomize.toolkit.fluxcd.io/v1", "apiVersion")
	setNestedString(ks, "./clusters/production", "spec", "path")
	setNestedString(ks, "GitRepository", "spec", "sourceRef", "kind")
	setNestedString(ks, "flux-system", "spec", "sourceRef", "name")
	setCondition(ks, "True", "ReconciliationSucceeded", "Applied revision: v1.0.0", "2025-01-01T00:00:00Z")

	source := makeObj("GitRepository", "flux-system", "flux-system")
	setNestedString(source, "source.toolkit.fluxcd.io/v1", "apiVersion")
	setNestedString(source, "ssh://git@github.com/org/repo", "spec", "url")
	setCondition(source, "True", "Succeeded", "stored artifact", "2025-01-01T00:00:00Z")

	r := newTestReporter(scheme, &ks, &source)
	syncStatus, err := r.getSyncStatus(ctx, crds)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(syncStatus).ToNot(BeNil())
	g.Expect(syncStatus.Ready).To(BeTrue())
	g.Expect(syncStatus.Path).To(Equal("./clusters/production"))
	g.Expect(syncStatus.Source).To(Equal("ssh://git@github.com/org/repo"))
	g.Expect(syncStatus.ID).To(Equal("kustomization/flux-system"))
	g.Expect(syncStatus.Status).To(ContainSubstring("Applied revision"))
}

func TestGetSyncStatus_Suspended(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	crds := []metav1.GroupVersionKind{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
	}

	ks := makeObj("Kustomization", "flux-system", "flux-system")
	setNestedString(ks, "kustomize.toolkit.fluxcd.io/v1", "apiVersion")
	setNestedBool(ks, true, "spec", "suspend")
	setCondition(ks, "True", "ReconciliationSucceeded", "Applied revision: v1.0.0", "2025-01-01T00:00:00Z")

	r := newTestReporter(scheme, &ks)
	syncStatus, err := r.getSyncStatus(ctx, crds)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(syncStatus).ToNot(BeNil())
	g.Expect(syncStatus.Status).To(HavePrefix("Suspended"))
}

func TestGetSyncStatus_SourceFailing(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	crds := []metav1.GroupVersionKind{
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Kind: "Kustomization"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Kind: "GitRepository"},
	}

	ks := makeObj("Kustomization", "flux-system", "flux-system")
	setNestedString(ks, "kustomize.toolkit.fluxcd.io/v1", "apiVersion")
	setNestedString(ks, "GitRepository", "spec", "sourceRef", "kind")
	setNestedString(ks, "flux-system", "spec", "sourceRef", "name")
	setCondition(ks, "True", "ReconciliationSucceeded", "Applied revision: v1.0.0", "2025-01-01T00:00:00Z")

	source := makeObj("GitRepository", "flux-system", "flux-system")
	setNestedString(source, "source.toolkit.fluxcd.io/v1", "apiVersion")
	setNestedString(source, "ssh://git@github.com/org/repo", "spec", "url")
	setCondition(source, "False", "AuthenticationFailed", "SSH key expired", "2025-01-01T00:00:00Z")

	r := newTestReporter(scheme, &ks, &source)
	syncStatus, err := r.getSyncStatus(ctx, crds)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(syncStatus).ToNot(BeNil())
	g.Expect(syncStatus.Ready).To(BeFalse())
	g.Expect(syncStatus.Status).To(ContainSubstring("SSH key expired"))
}
