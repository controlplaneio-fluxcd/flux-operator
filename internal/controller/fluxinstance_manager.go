// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimectrl "github.com/fluxcd/pkg/runtime/controller"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// FluxInstanceReconcilerOptions contains options for the reconciler.
type FluxInstanceReconcilerOptions struct {
	RateLimiter             workqueue.TypedRateLimiter[reconcile.Request]
	DisableWaitInterruption bool
}

// SetupWithManager sets up the controller with the Manager.
func (r *FluxInstanceReconciler) SetupWithManager(mgr ctrl.Manager, opts FluxInstanceReconcilerOptions) error {
	var blder *builder.Builder
	var toComplete reconcile.TypedReconciler[reconcile.Request]

	pred := predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)

	if opts.DisableWaitInterruption {
		toComplete = r
		blder = ctrl.NewControllerManagedBy(mgr).
			For(&fluxcdv1.FluxInstance{}, builder.WithPredicates(pred))
	} else {
		wr := runtimectrl.WrapReconciler(r)
		toComplete = wr
		blder = runtimectrl.NewControllerManagedBy(mgr, wr).
			For(&fluxcdv1.FluxInstance{}, pred).Builder
	}

	return blder.
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(toComplete)
}
