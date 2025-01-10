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

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetReconcilerOptions contains options for the reconciler.
type ResourceSetReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(mgr ctrl.Manager, opts ResourceSetReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1.ResourceSet{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
				),
			)).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(r)
}
