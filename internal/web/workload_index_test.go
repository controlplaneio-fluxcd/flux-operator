// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

func TestBuildWorkloadIndex(t *testing.T) {
	g := NewWithT(t)

	input := []reporter.WorkloadRef{
		{Name: "Web-1", Kind: "Deployment", Namespace: "default"},
		{Name: "Db", Kind: "StatefulSet", Namespace: "team-a"},
		{Name: "Agent", Kind: "DaemonSet", Namespace: "default"},
	}

	out := buildWorkloadIndex(input)
	g.Expect(out).To(HaveLen(3))

	// Sorted by kind, namespace, then name; names lowercased.
	g.Expect(out[0].Kind).To(Equal("DaemonSet"))
	g.Expect(out[0].Name).To(Equal("agent"))
	g.Expect(out[1].Kind).To(Equal("Deployment"))
	g.Expect(out[1].Name).To(Equal("web-1"))
	g.Expect(out[2].Kind).To(Equal("StatefulSet"))
	g.Expect(out[2].Name).To(Equal("db"))

	// Input is not mutated.
	g.Expect(input[0].Name).To(Equal("Web-1"))
}

func TestBuildWorkloadIndex_Empty(t *testing.T) {
	g := NewWithT(t)

	g.Expect(buildWorkloadIndex(nil)).To(BeNil())
	g.Expect(buildWorkloadIndex([]reporter.WorkloadRef{})).To(BeNil())
}

func TestWorkloadIndex_Update(t *testing.T) {
	g := NewWithT(t)

	idx := &WorkloadIndex{}
	g.Expect(idx.updatedAt.IsZero()).To(BeTrue())

	idx.Update([]reporter.WorkloadRef{
		{Name: "web", Kind: "Deployment", Namespace: "default"},
	})

	g.Expect(idx.updatedAt.IsZero()).To(BeFalse())
	g.Expect(idx.workloads).To(HaveLen(1))
}

func TestWorkloadIndex_SearchWorkloads_Filters(t *testing.T) {
	g := NewWithT(t)

	idx := &WorkloadIndex{}
	idx.Update([]reporter.WorkloadRef{
		{Name: "podinfo", Kind: "Deployment", Namespace: "team-a", LastReconciled: metav1.Now()},
		{Name: "podinfo-cache", Kind: "StatefulSet", Namespace: "team-a", LastReconciled: metav1.Now()},
		{Name: "backup", Kind: "CronJob", Namespace: "team-b", LastReconciled: metav1.Now()},
	})

	// Name wildcard.
	results := idx.SearchWorkloads(nil, "", "*podinfo*", "", 0)
	g.Expect(results).To(HaveLen(2))

	// Kind filter.
	results = idx.SearchWorkloads(nil, "CronJob", "", "", 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("backup"))

	// Namespace filter.
	results = idx.SearchWorkloads(nil, "", "", "team-a", 0)
	g.Expect(results).To(HaveLen(2))

	// Combined filters.
	results = idx.SearchWorkloads(nil, "Deployment", "*podinfo*", "team-a", 0)
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Name).To(Equal("podinfo"))
}

func TestWorkloadIndex_SearchWorkloads_NoStatusFilterArgument(t *testing.T) {
	// The workload index intentionally has no status filter: workloads carry
	// only the parent reconciler's status badge, which is not a filterable
	// dimension. This test simply documents that SearchWorkloads has no status
	// parameter by returning all entries regardless of ReconcilerStatus.
	g := NewWithT(t)

	idx := &WorkloadIndex{}
	idx.Update([]reporter.WorkloadRef{
		{Name: "a", Kind: "Deployment", Namespace: "default", ReconcilerStatus: reporter.StatusReady, LastReconciled: metav1.Now()},
		{Name: "b", Kind: "Deployment", Namespace: "default", ReconcilerStatus: reporter.StatusFailed, LastReconciled: metav1.Now()},
	})

	results := idx.SearchWorkloads(nil, "", "", "", 0)
	g.Expect(results).To(HaveLen(2))
}

func TestWorkloadIndex_SearchWorkloads_SortedByLastReconciledWithTiebreaker(t *testing.T) {
	g := NewWithT(t)

	older := metav1.Date(2025, 1, 1, 0, 0, 0, 0, metav1.Now().Location())
	newer := metav1.Date(2025, 6, 1, 0, 0, 0, 0, metav1.Now().Location())

	idx := &WorkloadIndex{}
	idx.Update([]reporter.WorkloadRef{
		// Same (newer) timestamp: tiebreaker (namespace, name) decides order.
		{Name: "zeta", Kind: "Deployment", Namespace: "team-b", LastReconciled: newer},
		{Name: "alpha", Kind: "Deployment", Namespace: "team-b", LastReconciled: newer},
		{Name: "gamma", Kind: "Deployment", Namespace: "team-a", LastReconciled: newer},
		// Older timestamp sorts last.
		{Name: "old", Kind: "Deployment", Namespace: "team-a", LastReconciled: older},
	})

	results := idx.SearchWorkloads(nil, "", "", "", 0)
	g.Expect(results).To(HaveLen(4))

	// Newest first, with (namespace, name) tiebreaker among equal timestamps.
	g.Expect(results[0].Namespace).To(Equal("team-a"))
	g.Expect(results[0].Name).To(Equal("gamma"))
	g.Expect(results[1].Namespace).To(Equal("team-b"))
	g.Expect(results[1].Name).To(Equal("alpha"))
	g.Expect(results[2].Namespace).To(Equal("team-b"))
	g.Expect(results[2].Name).To(Equal("zeta"))
	g.Expect(results[3].Name).To(Equal("old"))
}

func TestWorkloadIndex_SearchWorkloads_Limit(t *testing.T) {
	g := NewWithT(t)

	oldest := metav1.Date(2025, 1, 1, 0, 0, 0, 0, metav1.Now().Location())
	middle := metav1.Date(2025, 6, 1, 0, 0, 0, 0, metav1.Now().Location())
	newest := metav1.Date(2025, 12, 1, 0, 0, 0, 0, metav1.Now().Location())

	idx := &WorkloadIndex{}
	idx.Update([]reporter.WorkloadRef{
		{Name: "old", Kind: "Deployment", Namespace: "default", LastReconciled: oldest},
		{Name: "new", Kind: "Deployment", Namespace: "default", LastReconciled: newest},
		{Name: "mid", Kind: "Deployment", Namespace: "default", LastReconciled: middle},
	})

	results := idx.SearchWorkloads(nil, "", "", "", 2)
	g.Expect(results).To(HaveLen(2))
	g.Expect(results[0].Name).To(Equal("new"))
	g.Expect(results[1].Name).To(Equal("mid"))
}

func TestWorkloadIndex_SearchWorkloads_AllowedNamespaces(t *testing.T) {
	g := NewWithT(t)

	idx := &WorkloadIndex{}
	idx.Update([]reporter.WorkloadRef{
		{Name: "a", Kind: "Deployment", Namespace: "team-a", LastReconciled: metav1.Now()},
		{Name: "b", Kind: "Deployment", Namespace: "team-b", LastReconciled: metav1.Now()},
		{Name: "c", Kind: "Deployment", Namespace: "team-c", LastReconciled: metav1.Now()},
	})

	// Only team-a and team-c visible.
	results := idx.SearchWorkloads([]string{"team-a", "team-c"}, "", "", "", 0)
	g.Expect(results).To(HaveLen(2))
	for _, r := range results {
		g.Expect(r.Namespace).To(BeElementOf("team-a", "team-c"))
	}

	// nil allowedNamespaces means cluster-wide access.
	results = idx.SearchWorkloads(nil, "", "", "", 0)
	g.Expect(results).To(HaveLen(3))

	// Empty non-nil allowedNamespaces means no access.
	results = idx.SearchWorkloads([]string{}, "", "", "", 0)
	g.Expect(results).To(BeEmpty())
}

func TestWorkloadIndex_SearchWorkloads_EmptyIndex(t *testing.T) {
	g := NewWithT(t)

	idx := &WorkloadIndex{}
	results := idx.SearchWorkloads(nil, "", "*x*", "", 10)
	g.Expect(results).To(BeEmpty())
}
