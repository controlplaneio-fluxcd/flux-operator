// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/jitter"
	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	msgInProgress    = "Reconciliation in progress"
	msgInitSuspended = "Initialized with reconciliation suspended"
	msgTerminalError = "Reconciliation failed terminally due to configuration error"
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

// requeueAfter returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations or zero if not set.
// If the interval is greater than zero, it applies jitter to the requeue time.
func requeueAfter(obj fluxcdv1.FluxObject) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
		return jitter.JitteredRequeueInterval(result)
	}

	return result
}

// reconcileMessage returns a message indicating the reconciliation has finished
func reconcileMessage(t time.Time) string {
	return fmt.Sprintf("Reconciliation finished in %s", fmtDuration(t))
}

// uninstallMessage returns a message indicating the uninstallation has finished
func uninstallMessage(t time.Time) string {
	return fmt.Sprintf("Uninstallation compleated in %s", fmtDuration(t))
}

// fmtDuration returns a human-readable string
// representation of the time duration.
func fmtDuration(t time.Time) string {
	if time.Since(t) < time.Second {
		return time.Since(t).Round(time.Millisecond).String()
	} else {
		return time.Since(t).Round(time.Second).String()
	}
}

// aggregateNotReadyStatus returns the Ready condition message of the Flux resources in a failed state.
func aggregateNotReadyStatus(ctx context.Context, kubeClient client.Client, objects []*unstructured.Unstructured) string {
	var result strings.Builder
	for _, res := range objects {
		if strings.HasSuffix(res.GetObjectKind().GroupVersionKind().Group, ".fluxcd.io") {
			if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(res), res); err == nil {
				if obj, err := status.GetObjectWithConditions(res.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status != corev1.ConditionTrue {
							result.WriteString(fmt.Sprintf("%s status: %s\n", ssautil.FmtUnstructured(res), cond.Message))
						}
					}
				}
			}
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// takeOwnershipFrom returns a list of field managers that should be used
// for taking ownership of resources from other controllers and tools.
// By default, it includes managers for kustomize-controller, helm, and kubectl.
func takeOwnershipFrom(managers []string) []ssa.FieldManager {
	fieldManagers := []ssa.FieldManager{ //nolint:prealloc
		{
			Name:          "kustomize-controller",
			OperationType: metav1.ManagedFieldsOperationApply,
			ExactMatch:    true,
		},
		{
			Name:          "helm",
			OperationType: metav1.ManagedFieldsOperationUpdate,
			ExactMatch:    true,
		},
		{
			// to undo changes made with 'kubectl apply'
			Name:          "kubectl",
			OperationType: metav1.ManagedFieldsOperationUpdate,
		},
		{
			// to undo changes made with 'kubectl apply --server-side'
			Name:          "before-first-apply",
			OperationType: metav1.ManagedFieldsOperationUpdate,
		},
		{
			// to undo changes made with 'kubectl apply --server-side --force-conflicts'
			Name:          "kubectl",
			OperationType: metav1.ManagedFieldsOperationApply,
		},
	}

	for _, manager := range managers {
		fieldManagers = append(fieldManagers,
			ssa.FieldManager{
				Name:          manager,
				OperationType: metav1.ManagedFieldsOperationApply,
				ExactMatch:    true,
			},
			ssa.FieldManager{
				Name:          manager,
				OperationType: metav1.ManagedFieldsOperationUpdate,
				ExactMatch:    true,
			},
		)
	}

	return fieldManagers
}

// returnHealthChecksCanceled logs, sends an event, marks the object and returns the appropriate
// ctrl.Result for when a new reconciliation request is detected during health checks.
func returnHealthChecksCanceled(ctx context.Context, obj conditions.Setter,
	qes *controller.QueueEventSource, recorder kuberecorder.EventRecorder) (ctrl.Result, error) {

	ctrl.LoggerFrom(ctx).Info("New reconciliation triggered, canceling health checks", "trigger", qes)

	recorder.Eventf(obj, corev1.EventTypeNormal, meta.HealthCheckCanceledReason,
		"Health checks canceled due to new reconciliation triggered by %s/%s/%s",
		qes.Kind, qes.Namespace, qes.Name)

	conditions.MarkFalse(obj,
		meta.ReadyCondition,
		meta.HealthCheckCanceledReason,
		"New reconciliation triggered by %s/%s/%s", qes.Kind, qes.Namespace, qes.Name)

	// No need to return an error here since there's already a new reconciliation request in the queue.
	return ctrl.Result{}, nil
}
