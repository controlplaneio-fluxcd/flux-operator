// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	msgInProgress    = "Reconciliation in progress"
	msgInitSuspended = "Initialized with reconciliation suspended"
)

// initializeObjectStatus initializes the FluxObject by adding a finalizer and setting
// the status conditions based on whether reconciliation is disabled or not.
func initializeObjectStatus(obj fluxcdv1.FluxObject) {
	controllerutil.AddFinalizer(obj, fluxcdv1.Finalizer)
	if obj.IsDisabled() {
		conditions.MarkTrue(obj,
			meta.ReadyCondition,
			fluxcdv1.ReconciliationDisabledReason,
			"%s", msgInitSuspended)
	} else {
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			"%s", msgInProgress)
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			"%s", msgInProgress)
	}
}

// finalizeObjectStatus updates the status of the FluxObject after reconciliation
// by setting the last handled reconcile time and removing kstatus stale conditions.
func finalizeObjectStatus(obj fluxcdv1.FluxObject) {
	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.SetLastHandledReconcileAt(v)
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Remove the Reconciling condition.
	if conditions.IsTrue(obj, meta.ReadyCondition) || conditions.IsTrue(obj, meta.StalledCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
	}
}
