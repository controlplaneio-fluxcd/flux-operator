// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"cmp"
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func (r *FluxStatusReporter) getReconcilersStatus(ctx context.Context, crds []metav1.GroupVersionKind) ([]fluxcdv1.FluxReconcilerStatus, error) {
	var multiErr error
	metricList := make([]prometheus.Labels, 0)
	resStats := make([]fluxcdv1.FluxReconcilerStatus, len(crds))
	for i, gvk := range crds {
		var total int
		var suspended int
		var failing int
		var totalSize int64

		list := unstructured.UnstructuredList{
			Object: map[string]any{
				"apiVersion": gvk.Group + "/" + gvk.Version,
				"kind":       gvk.Kind,
			},
		}

		if err := r.List(ctx, &list, client.InNamespace("")); err == nil {
			total = len(list.Items)
			for _, item := range list.Items {
				metricList = append(metricList, fluxLabelsToValues(item))

				if s, _, _ := unstructured.NestedBool(item.Object, "spec", "suspend"); s {
					suspended++
				}

				if obj, err := status.GetObjectWithConditions(item.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse {
							failing++
						}
					}
				}

				if size, found, _ := unstructured.NestedInt64(item.Object, "status", "artifact", "size"); found {
					totalSize += size
				}
			}
		} else {
			multiErr = kerrors.NewAggregate([]error{multiErr, err})
		}

		resStats[i] = fluxcdv1.FluxReconcilerStatus{
			APIVersion: gvk.Group + "/" + gvk.Version,
			Kind:       gvk.Kind,
			Stats: fluxcdv1.FluxReconcilerStats{
				Running:   total - suspended,
				Failing:   failing,
				Suspended: suspended,
				TotalSize: formatSize(totalSize),
			},
		}
	}

	// Record the per resource metrics.
	ResetMetrics("FluxResource")
	for _, labels := range metricList {
		metrics["FluxResource"].With(labels).Set(1)
	}

	opStats, err := r.getOperatorReconcilersStatus(ctx)
	if err != nil {
		multiErr = kerrors.NewAggregate([]error{multiErr, err})
	} else {
		resStats = append(resStats, opStats...)
	}

	slices.SortStableFunc(resStats, func(i, j fluxcdv1.FluxReconcilerStatus) int {
		return cmp.Compare(i.APIVersion+i.Kind, j.APIVersion+j.Kind)
	})

	return resStats, multiErr
}

func (r *FluxStatusReporter) getOperatorReconcilersStatus(ctx context.Context) ([]fluxcdv1.FluxReconcilerStatus, error) {
	var multiErr error
	crds := []schema.GroupVersionKind{
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind),
		fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind),
	}
	resStats := make([]fluxcdv1.FluxReconcilerStatus, len(crds))
	for i, gvk := range crds {
		var total int
		var suspended int
		var failing int

		list := unstructured.UnstructuredList{
			Object: map[string]any{
				"apiVersion": gvk.Group + "/" + gvk.Version,
				"kind":       gvk.Kind,
			},
		}

		if err := r.List(ctx, &list, client.InNamespace("")); err == nil {
			total = len(list.Items)

			for _, item := range list.Items {
				if ssautil.AnyInMetadata(&item, map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
					suspended++
				}

				if obj, err := status.GetObjectWithConditions(item.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse {
							failing++
						}
					}
				}
			}
		} else {
			multiErr = kerrors.NewAggregate([]error{multiErr, err})
		}

		resStats[i] = fluxcdv1.FluxReconcilerStatus{
			APIVersion: gvk.Group + "/" + gvk.Version,
			Kind:       gvk.Kind,
			Stats: fluxcdv1.FluxReconcilerStats{
				Running:   total - suspended,
				Failing:   failing,
				Suspended: suspended,
			},
		}
	}

	return resStats, multiErr
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
