// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetInputProviderReconciler reconciles a ResourceSetInputProvider object
type ResourceSetInputProviderReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	StatusManager string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesetinputproviders/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceSetInputProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	// Finalise the reconciliation and report the results.
	defer func() {
		if err := r.finalizeStatus(ctx, obj, patcher); err != nil {
			log.Error(err, "failed to update status")
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// Uninstall if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.uninstall(ctx, obj)
	}

	// Add the finalizer if it does not exist.
	if !controllerutil.ContainsFinalizer(obj, fluxcdv1.Finalizer) {
		log.Info("Adding finalizer", "finalizer", fluxcdv1.Finalizer)
		controllerutil.AddFinalizer(obj, fluxcdv1.Finalizer)
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			"%s", msgInProgress)
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			"%s", msgInProgress)
		return ctrl.Result{Requeue: true}, nil
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		log.Error(errors.New("can't reconcile instance"), fluxcdv1.ReconciliationDisabledMessage)
		r.Event(obj, corev1.EventTypeWarning, fluxcdv1.ReconciliationDisabledReason, fluxcdv1.ReconciliationDisabledMessage)
		return ctrl.Result{}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *ResourceSetInputProviderReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	// Mark the object as reconciling.
	msg := "Reconciliation in progress"
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		"%s", msg)
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		"%s", msg)
	if err := r.patch(ctx, obj, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	// Mark the object as ready and set the last applied revision.
	msg = fmt.Sprintf("Reconciliation finished in %s", fmtDuration(reconcileStart))
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfterResourceSetInputProvider(obj), nil
}

// finalizeStatus updates the object status and conditions.
func (r *ResourceSetInputProviderReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) error {
	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.LastHandledReconcileAt = v
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

	// Patch finalizers, status and conditions.
	return r.patch(ctx, obj, patcher)
}

// uninstall deletes all the resources managed by the ResourceSetInputProvider.
//
//nolint:unparam
func (r *ResourceSetInputProviderReconciler) uninstall(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider) (ctrl.Result, error) {

	// Release the object to be garbage collected.
	controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// patch updates the object status, conditions and finalizers.
func (r *ResourceSetInputProviderReconciler) patch(ctx context.Context,
	obj *fluxcdv1.ResourceSetInputProvider,
	patcher *patch.SerialPatcher) (retErr error) {
	// Configure the runtime patcher.
	ownedConditions := []string{
		meta.ReadyCondition,
		meta.ReconcilingCondition,
		meta.StalledCondition,
	}
	patchOpts := []patch.Option{
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithForceOverwriteConditions{},
		patch.WithFieldOwner(r.StatusManager),
	}

	// Patch the object status, conditions and finalizers.
	if err := patcher.Patch(ctx, obj, patchOpts...); err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
		}
		retErr = kerrors.NewAggregate([]error{retErr, err})
		if retErr != nil {
			return retErr
		}
	}

	return nil
}

// requeueAfterResourceSetInputProvider returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations.
func requeueAfterResourceSetInputProvider(obj *fluxcdv1.ResourceSetInputProvider) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result
}
