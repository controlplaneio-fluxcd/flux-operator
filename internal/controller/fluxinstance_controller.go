// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	fluxcdv1alpha1 "github.com/controlplaneio-fluxcd/fluxcd-operator/api/v1alpha1"
)

// FluxInstanceReconciler reconciles a FluxInstance object
type FluxInstanceReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme        *runtime.Scheme
	StatusManager string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FluxInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	obj := &fluxcdv1alpha1.FluxInstance{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	// Finalise the reconciliation and report the results.
	defer func() {
		if err := r.finalizeStatus(ctx, obj, patcher); err != nil {
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}

		if conditions.IsReady(obj) {
			msg := fmt.Sprintf("Reconciliation finished in %s", time.Since(reconcileStart).String())
			log.Info(msg, "generation", obj.GetGeneration())
			r.EventRecorder.Event(obj, corev1.EventTypeNormal, meta.SucceededReason, msg)
		}
	}()

	// Reconcile the object.
	reconcileErr := r.reconcile(ctx, obj, patcher)
	if reconcileErr != nil {
		return ctrl.Result{}, reconcileErr
	}

	// Mark the object as ready.
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.SucceededReason,
		fmt.Sprintf("Applied revision: %v", obj.GetGeneration()))

	return ctrl.Result{}, nil
}

func (r *FluxInstanceReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
	patcher *patch.SerialPatcher) error {

	// Mark the object as reconciling.
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		"Reconciliation in progress")
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		"Reconciliation in progress")
	if err := r.patch(ctx, obj, patcher); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (r *FluxInstanceReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
	patcher *patch.SerialPatcher) error {
	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.LastHandledReconcileAt = v
	}

	// Remove the Reconciling condition and update the observed generation
	// if the reconciliation was successful.
	if conditions.IsTrue(obj, meta.ReadyCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
		obj.Status.ObservedGeneration = obj.Generation
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Patch finalizers, status and conditions.
	return r.patch(ctx, obj, patcher)
}

func (r *FluxInstanceReconciler) patch(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
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

// SetupWithManager sets up the controller with the Manager.
func (r *FluxInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1alpha1.FluxInstance{},
			builder.WithPredicates(
				predicate.GenerationChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
			)).Complete(r)
}
