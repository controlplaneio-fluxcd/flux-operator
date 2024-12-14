// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func (r *FluxStatusReporter) getSyncStatus(ctx context.Context, crds []metav1.GroupVersionKind) (*fluxcdv1.FluxSyncStatus, error) {
	syncName := r.getSyncNameFromInstance(ctx)

	syncKind := "Kustomization"
	syncGKV := gvkFor(syncKind, crds)
	if syncGKV == nil {
		return nil, fmt.Errorf("source kind %s not found", syncKind)
	}

	syncObj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": syncGKV.Group + "/" + syncGKV.Version,
			"kind":       syncKind,
		},
	}

	if err := r.Get(ctx, client.ObjectKey{
		Namespace: r.namespace,
		Name:      syncName,
	}, &syncObj); err != nil {
		if apiErrors.IsNotFound(err) {
			// No sync configured, return empty status.
			return nil, nil
		}
		return nil, fmt.Errorf("failed to assert sync status: %w", err)
	}

	syncStatus := &fluxcdv1.FluxSyncStatus{
		ID:     fmt.Sprintf("%s/%s", strings.ToLower(syncKind), syncName),
		Ready:  false,
		Status: "not initialized",
	}

	// Read spec.path from the sync object.
	if path, found, _ := unstructured.NestedString(syncObj.Object, "spec", "path"); found {
		syncStatus.Path = path
	}

	// Set sync readiness based on the Kustomization object conditions.
	if obj, err := status.GetObjectWithConditions(syncObj.Object); err == nil {
		for _, cond := range obj.Status.Conditions {
			if cond.Type == meta.ReadyCondition {
				syncStatus.Ready = cond.Status != corev1.ConditionFalse
				syncStatus.Status = cond.Message
			}
		}
	}

	// Set source URL and readiness based on the source object conditions.
	if sourceKind, found, _ := unstructured.NestedString(syncObj.Object, "spec", "sourceRef", "kind"); found {
		sourceGVK := gvkFor(sourceKind, crds)
		if sourceGVK == nil {
			return nil, fmt.Errorf("source kind %s not found", sourceKind)
		}

		sourceObj := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": sourceGVK.Group + "/" + sourceGVK.Version,
				"kind":       sourceGVK.Kind,
			},
		}

		if err := r.Get(ctx, client.ObjectKey{
			Namespace: r.namespace,
			Name:      syncName,
		}, &sourceObj); err == nil {
			if sourceURL, found, _ := unstructured.NestedString(sourceObj.Object, "spec", "url"); found {
				syncStatus.Source = sourceURL
			}

			if obj, err := status.GetObjectWithConditions(sourceObj.Object); err == nil {
				for _, cond := range obj.Status.Conditions {
					if cond.Type == meta.ReadyCondition && cond.Status == corev1.ConditionFalse {
						syncStatus.Ready = false
						// Append source error status to sync status.
						syncStatus.Status += " " + cond.Message
					}
				}
			}
		}
	}
	return syncStatus, nil
}

func (r *FluxStatusReporter) getSyncNameFromInstance(ctx context.Context) string {
	syncName := r.namespace
	instanceList := &fluxcdv1.FluxInstanceList{}
	if err := r.List(ctx, instanceList, client.InNamespace(r.namespace)); err == nil {
		if len(instanceList.Items) > 0 {
			if s := instanceList.Items[0].Spec.Sync; s != nil && s.Name != "" {
				syncName = s.Name
			}
		}
	}

	return syncName
}
