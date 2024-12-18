// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// FluxInstanceArtifactReconcilerOptions contains options for the reconciler.
type FluxInstanceArtifactReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// ArtifactReconciliationConfigurationChangedPredicate contains the logic
// to determine if the annotations of a FluxInstance object relevant to the
// artifact reconciliation have changed.
type ArtifactReconciliationConfigurationChangedPredicate struct {
	predicate.Funcs
}

// SetupWithManager sets up the controller with the Manager.
func (r *FluxInstanceArtifactReconciler) SetupWithManager(mgr ctrl.Manager, opts FluxInstanceArtifactReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("fluxinstance_artifact").
		Watches(&fluxcdv1.FluxInstance{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(ArtifactReconciliationConfigurationChangedPredicate{})).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(r)
}

func (ArtifactReconciliationConfigurationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	// Start reconciliation if the object was disabled and is now enabled.
	oldObj := e.ObjectOld.(*fluxcdv1.FluxInstance)
	newObj := e.ObjectNew.(*fluxcdv1.FluxInstance)
	if oldObj.IsDisabled() && !newObj.IsDisabled() {
		return true
	}

	// Trigger reconciliation if the artifact interval has changed.
	if oldObj.GetArtifactInterval() != newObj.GetArtifactInterval() {
		return true
	}

	return false
}
