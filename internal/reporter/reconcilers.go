// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type reconcilerStats struct {
	total     int
	suspended int
	failing   int
	totalSize int64
}

type ReconcilerStatsByNamespace struct {
	apiVersion string
	kind       string
	stats      map[string]*reconcilerStats
}

// FilterReconcilerStatsByNamespaces filters the reconciler stats by the provided namespaces.
func FilterReconcilerStatsByNamespaces(statsByNamespace []ReconcilerStatsByNamespace, namespaces []string) []fluxcdv1.FluxReconcilerStatus {
	namespaceMap := make(map[string]struct{}, len(namespaces))
	for _, ns := range namespaces {
		namespaceMap[ns] = struct{}{}
	}

	stats := make([]fluxcdv1.FluxReconcilerStatus, 0, len(statsByNamespace))
	for _, res := range statsByNamespace {
		var globalStats reconcilerStats
		for ns, nsStats := range res.stats {
			if _, exists := namespaceMap[ns]; !exists {
				continue
			}
			globalStats.total += nsStats.total
			globalStats.suspended += nsStats.suspended
			globalStats.failing += nsStats.failing
			globalStats.totalSize += nsStats.totalSize
		}
		stats = append(stats, fluxcdv1.FluxReconcilerStatus{
			APIVersion: res.apiVersion,
			Kind:       res.kind,
			Stats: fluxcdv1.FluxReconcilerStats{
				Running:   globalStats.total - globalStats.suspended,
				Failing:   globalStats.failing,
				Suspended: globalStats.suspended,
				TotalSize: formatSize(globalStats.totalSize),
			},
		})
	}
	return stats
}

func (r *FluxStatusReporter) getReconcilersStatus(ctx context.Context,
	crds []metav1.GroupVersionKind) ([]fluxcdv1.FluxReconcilerStatus, []ReconcilerStatsByNamespace, error) {

	var multiErr error
	metricList := make([]prometheus.Labels, 0)
	resStats := make([]fluxcdv1.FluxReconcilerStatus, len(crds))
	statsByNamespace := make([]ReconcilerStatsByNamespace, len(crds))
	for i, gvk := range crds {
		apiVersion := gvk.Group + "/" + gvk.Version

		statsByNamespace[i] = ReconcilerStatsByNamespace{
			apiVersion: apiVersion,
			kind:       gvk.Kind,
			stats:      make(map[string]*reconcilerStats),
		}

		var globalStats reconcilerStats

		list := unstructured.UnstructuredList{
			Object: map[string]any{
				"apiVersion": apiVersion,
				"kind":       gvk.Kind,
			},
		}

		if err := r.List(ctx, &list, client.InNamespace("")); err == nil {
			globalStats.total = len(list.Items)
			for _, item := range list.Items {
				metricList = append(metricList, fluxLabelsToValues(item))

				ns := item.GetNamespace()
				if _, exists := statsByNamespace[i].stats[ns]; !exists {
					statsByNamespace[i].stats[ns] = &reconcilerStats{}
				}
				nsStats := statsByNamespace[i].stats[ns]
				nsStats.total++

				if s, _, _ := unstructured.NestedBool(item.Object, "spec", "suspend"); s {
					globalStats.suspended++
					nsStats.suspended++
				}

				if obj, err := status.GetObjectWithConditions(item.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse {
							globalStats.failing++
							nsStats.failing++
						}
					}
				}

				if size, found, _ := unstructured.NestedInt64(item.Object, "status", "artifact", "size"); found {
					globalStats.totalSize += size
					nsStats.totalSize += size
				}
			}
		} else {
			multiErr = kerrors.NewAggregate([]error{multiErr, err})
		}

		resStats[i] = fluxcdv1.FluxReconcilerStatus{
			APIVersion: apiVersion,
			Kind:       gvk.Kind,
			Stats: fluxcdv1.FluxReconcilerStats{
				Running:   globalStats.total - globalStats.suspended,
				Failing:   globalStats.failing,
				Suspended: globalStats.suspended,
				TotalSize: formatSize(globalStats.totalSize),
			},
		}
	}

	// Record the per resource metrics.
	ResetMetrics("FluxResource")
	for _, labels := range metricList {
		metrics["FluxResource"].With(labels).Set(1)
	}

	opStats, opStatsByNamespace, err := r.getOperatorReconcilersStatus(ctx)
	if err != nil {
		multiErr = kerrors.NewAggregate([]error{multiErr, err})
	} else {
		resStats = append(resStats, opStats...)
		statsByNamespace = append(statsByNamespace, opStatsByNamespace...)
	}

	slices.SortStableFunc(resStats, func(i, j fluxcdv1.FluxReconcilerStatus) int {
		return cmp.Compare(i.APIVersion+i.Kind, j.APIVersion+j.Kind)
	})
	slices.SortStableFunc(statsByNamespace, func(i, j ReconcilerStatsByNamespace) int {
		return cmp.Compare(i.apiVersion+i.kind, j.apiVersion+j.kind)
	})

	return resStats, statsByNamespace, multiErr
}

func (r *FluxStatusReporter) getOperatorReconcilersStatus(
	ctx context.Context) ([]fluxcdv1.FluxReconcilerStatus, []ReconcilerStatsByNamespace, error) {

	var multiErr error
	crds := []schema.GroupVersionKind{
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind),
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind),
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind),
	}
	resStats := make([]fluxcdv1.FluxReconcilerStatus, len(crds))
	statsByNamespace := make([]ReconcilerStatsByNamespace, len(crds))
	for i, gvk := range crds {
		apiVersion := gvk.Group + "/" + gvk.Version

		statsByNamespace[i] = ReconcilerStatsByNamespace{
			apiVersion: apiVersion,
			kind:       gvk.Kind,
			stats:      make(map[string]*reconcilerStats),
		}

		var globalStats reconcilerStats

		list := unstructured.UnstructuredList{
			Object: map[string]any{
				"apiVersion": apiVersion,
				"kind":       gvk.Kind,
			},
		}

		if err := r.List(ctx, &list, client.InNamespace("")); err == nil {
			globalStats.total = len(list.Items)

			for _, item := range list.Items {
				ns := item.GetNamespace()
				if _, exists := statsByNamespace[i].stats[ns]; !exists {
					statsByNamespace[i].stats[ns] = &reconcilerStats{}
				}
				nsStats := statsByNamespace[i].stats[ns]
				nsStats.total++

				if ssautil.AnyInMetadata(&item, map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
					globalStats.suspended++
					nsStats.suspended++
				}

				if obj, err := status.GetObjectWithConditions(item.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse {
							globalStats.failing++
							nsStats.failing++
						}
					}
				}
			}
		} else {
			multiErr = kerrors.NewAggregate([]error{multiErr, err})
		}

		resStats[i] = fluxcdv1.FluxReconcilerStatus{
			APIVersion: apiVersion,
			Kind:       gvk.Kind,
			Stats: fluxcdv1.FluxReconcilerStats{
				Running:   globalStats.total - globalStats.suspended,
				Failing:   globalStats.failing,
				Suspended: globalStats.suspended,
			},
		}
	}

	return resStats, statsByNamespace, multiErr
}

func formatSize(b int64) string {
	if b == 0 {
		return ""
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
