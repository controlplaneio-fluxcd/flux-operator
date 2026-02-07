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
