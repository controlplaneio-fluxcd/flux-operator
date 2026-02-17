// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

func TestBuildSearchIndex(t *testing.T) {
	g := NewWithT(t)

	input := []reporter.ResourceStatus{
		{Name: "App-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady},
		{Name: "App-2", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusFailed},
		{Name: "Nginx", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusReady},
	}

	resources := buildSearchIndex(input)
	g.Expect(resources).To(HaveLen(3))

	// Verify names are lowercased
	for _, rs := range resources {
		g.Expect(rs.Name).To(Equal(rs.Name), "names should be lowercased")
	}

	// Verify sorted by kind, namespace, then name
	g.Expect(resources[0].Name).To(Equal("nginx"))
	g.Expect(resources[0].Namespace).To(Equal("default"))
	g.Expect(resources[0].Kind).To(Equal("HelmRelease"))
	g.Expect(resources[0].Status).To(Equal(reporter.StatusReady))

	g.Expect(resources[1].Name).To(Equal("app-1"))
	g.Expect(resources[1].Namespace).To(Equal("default"))
	g.Expect(resources[1].Kind).To(Equal("Kustomization"))

	g.Expect(resources[2].Name).To(Equal("app-2"))
	g.Expect(resources[2].Namespace).To(Equal("team-a"))
	g.Expect(resources[2].Kind).To(Equal("Kustomization"))
	g.Expect(resources[2].Status).To(Equal(reporter.StatusFailed))

	// Verify input is not mutated
	g.Expect(input[0].Name).To(Equal("App-1"))
}

func TestBuildSearchIndex_Empty(t *testing.T) {
	g := NewWithT(t)

	g.Expect(buildSearchIndex(nil)).To(BeNil())
	g.Expect(buildSearchIndex([]reporter.ResourceStatus{})).To(BeNil())
}

func TestSearchIndex_Update(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	g.Expect(idx.updatedAt.IsZero()).To(BeTrue())

	resources := []reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady},
	}
	idx.Update(resources)

	g.Expect(idx.updatedAt.IsZero()).To(BeFalse())
	g.Expect(idx.resources).To(HaveLen(1))
}

func TestSearchIndex_SearchResources_NameFilter(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "flux-system", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-deploy", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "flux-monitoring", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Wildcard match
	results := idx.SearchResources(nil, "", "*flux*", "", "", 0)
	g.Expect(results).To(HaveLen(2))

	// Prefix match
	results = idx.SearchResources(nil, "", "flux*", "", "", 0)
	g.Expect(results).To(HaveLen(2))

	// Exact match
	results = idx.SearchResources(nil, "", "app-deploy", "", "", 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("app-deploy"))

	// No match
	results = idx.SearchResources(nil, "", "*nonexistent*", "", "", 0)
	g.Expect(results).To(BeEmpty())
}

func TestSearchIndex_SearchResources_NamespaceFilter(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-2", Kind: "Kustomization", Namespace: "team-b", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-3", Kind: "HelmRelease", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	results := idx.SearchResources(nil, "", "", "team-a", "", 0)
	g.Expect(results).To(HaveLen(2))
	for _, r := range results {
		g.Expect(r.Namespace).To(Equal("team-a"))
	}
}

func TestSearchIndex_SearchResources_KindFilter(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-2", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-3", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	results := idx.SearchResources(nil, "HelmRelease", "", "", "", 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Kind).To(Equal("HelmRelease"))
}

func TestSearchIndex_SearchResources_Limit(t *testing.T) {
	g := NewWithT(t)

	oldest := metav1.Date(2025, 1, 1, 0, 0, 0, 0, metav1.Now().Location())
	middle := metav1.Date(2025, 6, 1, 0, 0, 0, 0, metav1.Now().Location())
	newest := metav1.Date(2025, 12, 1, 0, 0, 0, 0, metav1.Now().Location())

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-old", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: oldest},
		{Name: "app-new", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: newest},
		{Name: "app-mid", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: middle},
	})

	// Limit 2 should return the 2 most recently reconciled
	results := idx.SearchResources(nil, "", "", "", "", 2)
	g.Expect(results).To(HaveLen(2))
	g.Expect(results[0].Name).To(Equal("app-new"))
	g.Expect(results[1].Name).To(Equal("app-mid"))
}

func TestSearchIndex_SearchResources_AllowedNamespaces(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-2", Kind: "Kustomization", Namespace: "team-b", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-3", Kind: "Kustomization", Namespace: "team-c", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Only team-a and team-c visible
	results := idx.SearchResources([]string{"team-a", "team-c"}, "", "", "", "", 0)
	g.Expect(results).To(HaveLen(2))
	for _, r := range results {
		g.Expect(r.Namespace).To(BeElementOf("team-a", "team-c"))
	}

	// nil allowedNamespaces means all namespaces visible (cluster-wide access)
	results = idx.SearchResources(nil, "", "", "", "", 0)
	g.Expect(results).To(HaveLen(3))

	// Empty non-nil allowedNamespaces means no access — should return nothing
	results = idx.SearchResources([]string{}, "", "", "", "", 0)
	g.Expect(results).To(BeEmpty())
}

func TestSearchIndex_SearchResources_CombinedFilters(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "flux-system", Kind: "Kustomization", Namespace: "flux-system", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "flux-monitoring", Kind: "HelmRelease", Namespace: "flux-system", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
		{Name: "flux-app", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "other-app", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
	})

	// Filter by name, kind, and namespace simultaneously
	results := idx.SearchResources(nil, "Kustomization", "*flux*", "team-a", "", 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("flux-app"))
}

func TestSearchIndex_SearchResources_EmptyIndex(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}

	results := idx.SearchResources(nil, "", "*flux*", "", "", 10)
	g.Expect(results).To(BeEmpty())
}

func TestSearchIndex_SearchResources_SortedByLastReconciled(t *testing.T) {
	g := NewWithT(t)

	older := metav1.Date(2025, 1, 1, 0, 0, 0, 0, metav1.Now().Location())
	newer := metav1.Date(2025, 6, 1, 0, 0, 0, 0, metav1.Now().Location())

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "old-app", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: older},
		{Name: "new-app", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: newer},
	})

	results := idx.SearchResources(nil, "", "", "", "", 0)
	g.Expect(results).To(HaveLen(2))
	g.Expect(results[0].Name).To(Equal("new-app"))
	g.Expect(results[1].Name).To(Equal("old-app"))
}

func TestSearchIndex_SearchResources_StatusFilter(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-ready", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-failed", Kind: "Kustomization", Namespace: "default", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
		{Name: "app-suspended", Kind: "HelmRelease", Namespace: "default", Status: reporter.StatusSuspended, LastReconciled: metav1.Now()},
	})

	// Filter by Ready status
	results := idx.SearchResources(nil, "", "", "", reporter.StatusReady, 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("app-ready"))

	// Filter by Failed status
	results = idx.SearchResources(nil, "", "", "", reporter.StatusFailed, 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("app-failed"))

	// No status filter returns all
	results = idx.SearchResources(nil, "", "", "", "", 0)
	g.Expect(results).To(HaveLen(3))
}

func TestSearchIndex_SearchResources_StatusWithOtherFilters(t *testing.T) {
	g := NewWithT(t)

	idx := &SearchIndex{}
	idx.Update([]reporter.ResourceStatus{
		{Name: "app-1", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "app-2", Kind: "Kustomization", Namespace: "team-a", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
		{Name: "app-3", Kind: "HelmRelease", Namespace: "team-a", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
		{Name: "app-4", Kind: "Kustomization", Namespace: "team-b", Status: reporter.StatusFailed, LastReconciled: metav1.Now()},
	})

	// Filter by kind + namespace + status
	results := idx.SearchResources(nil, "Kustomization", "", "team-a", reporter.StatusFailed, 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("app-2"))
}
