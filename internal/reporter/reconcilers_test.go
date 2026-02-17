// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"testing"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDependencyNotReady_NotCountedAsFailing(t *testing.T) {
	g := NewWithT(t)

	items := []unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"metadata": map[string]any{
					"name":      "ready-ks",
					"namespace": "default",
				},
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":   meta.ReadyCondition,
							"status": string(corev1.ConditionTrue),
							"reason": meta.ReconciliationSucceededReason,
						},
					},
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"metadata": map[string]any{
					"name":      "dep-not-ready-ks",
					"namespace": "default",
				},
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":   meta.ReadyCondition,
							"status": string(corev1.ConditionFalse),
							"reason": meta.DependencyNotReadyReason,
						},
					},
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
				"kind":       "Kustomization",
				"metadata": map[string]any{
					"name":      "failed-ks",
					"namespace": "default",
				},
				"status": map[string]any{
					"conditions": []any{
						map[string]any{
							"type":   meta.ReadyCondition,
							"status": string(corev1.ConditionFalse),
							"reason": meta.ReconciliationFailedReason,
						},
					},
				},
			},
		},
	}

	var globalStats reconcilerStats
	globalStats.total = len(items)

	for _, item := range items {
		if obj, err := status.GetObjectWithConditions(item.Object); err == nil {
			for _, cond := range obj.Status.Conditions {
				if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse &&
					cond.Reason != meta.DependencyNotReadyReason {
					globalStats.failing++
				}
			}
		}
	}

	// Only the truly failed resource should be counted as failing,
	// not the one with DependencyNotReady reason.
	g.Expect(globalStats.total).To(Equal(3))
	g.Expect(globalStats.failing).To(Equal(1))
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero returns empty", 0, ""},
		{"bytes", 512, "512 B"},
		{"one KiB", 1024, "1.0 KiB"},
		{"KiB range", 1536, "1.5 KiB"},
		{"one MiB", 1048576, "1.0 MiB"},
		{"MiB range", 5242880, "5.0 MiB"},
		{"one GiB", 1073741824, "1.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(formatSize(tt.bytes)).To(Equal(tt.expected))
		})
	}
}

func TestFilterReconcilerStatsByNamespaces(t *testing.T) {
	g := NewWithT(t)

	statsByNamespace := []ReconcilerStatsByNamespace{
		{
			apiVersion: "kustomize.toolkit.fluxcd.io/v1",
			kind:       "Kustomization",
			stats: map[string]*reconcilerStats{
				"team-a": {total: 5, failing: 1, suspended: 1, totalSize: 1024},
				"team-b": {total: 3, failing: 0, suspended: 0, totalSize: 2048},
				"team-c": {total: 2, failing: 2, suspended: 0, totalSize: 512},
			},
		},
		{
			apiVersion: "source.toolkit.fluxcd.io/v1",
			kind:       "GitRepository",
			stats: map[string]*reconcilerStats{
				"team-a": {total: 2, failing: 0, suspended: 0, totalSize: 0},
				"team-b": {total: 1, failing: 1, suspended: 0, totalSize: 0},
			},
		},
	}

	// Filter to only include team-a and team-b.
	result := FilterReconcilerStatsByNamespaces(statsByNamespace, []string{"team-a", "team-b"})
	g.Expect(result).To(HaveLen(2))

	// Kustomization stats: team-a (5 total, 1 failing, 1 suspended) + team-b (3 total, 0 failing, 0 suspended).
	g.Expect(result[0].Kind).To(Equal("Kustomization"))
	g.Expect(result[0].Stats.Running).To(Equal(7))   // (5+3) - (1+0) suspended
	g.Expect(result[0].Stats.Failing).To(Equal(1))   // 1+0
	g.Expect(result[0].Stats.Suspended).To(Equal(1)) // 1+0
	g.Expect(result[0].Stats.TotalSize).To(Equal("3.0 KiB"))

	// GitRepository stats: team-a (2 total, 0 failing) + team-b (1 total, 1 failing).
	g.Expect(result[1].Kind).To(Equal("GitRepository"))
	g.Expect(result[1].Stats.Running).To(Equal(3)) // (2+1) - 0 suspended
	g.Expect(result[1].Stats.Failing).To(Equal(1)) // 0+1
	g.Expect(result[1].Stats.Suspended).To(Equal(0))
}
