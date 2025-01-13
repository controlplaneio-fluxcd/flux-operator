// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetReconcilerOptions contains options for the reconciler.
type ResourceSetReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, opts ResourceSetReconcilerOptions) error {
	const inputsProviderIndexKey string = ".metadata.inputsProvider"

	if err := mgr.GetCache().IndexField(ctx, &fluxcdv1.ResourceSet{}, inputsProviderIndexKey,
		r.indexBy(fluxcdv1.ResourceSetInputProviderKind)); err != nil {
		return fmt.Errorf("failed setting index fields: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1.ResourceSet{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
				),
			)).
		Watches(
			&fluxcdv1.ResourceSetInputProvider{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeOf(inputsProviderIndexKey)),
			builder.WithPredicates(exportedInputsChangePredicate),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(r)
}

func (r *ResourceSetReconciler) requestsForChangeOf(indexKey string) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx)

		var list fluxcdv1.ResourceSetList
		if err := r.List(ctx, &list, client.MatchingFields{
			indexKey: client.ObjectKeyFromObject(obj).String(),
		}); err != nil {
			log.Error(err, "failed to list objects for provider change")
			return nil
		}

		reqs := make([]reconcile.Request, len(list.Items))
		for i, rset := range list.Items {
			reqs[i].NamespacedName = types.NamespacedName{Name: rset.Name, Namespace: rset.Namespace}
		}

		return reqs
	}
}

func (r *ResourceSetReconciler) indexBy(kind string) func(o client.Object) []string {
	return func(o client.Object) []string {
		rs, ok := o.(*fluxcdv1.ResourceSet)
		if !ok {
			return nil
		}

		if len(rs.Spec.InputsFrom) == 0 {
			return nil
		}

		results := make([]string, 0)
		for _, k := range rs.Spec.InputsFrom {
			if k.Kind == kind {
				ns := rs.GetNamespace()
				if k.Namespace != "" {
					ns = k.Namespace
				}
				results = append(results, fmt.Sprintf("%s/%s", ns, k.Name))
			}
		}

		return results
	}
}

var exportedInputsChangePredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldObj := e.ObjectOld.(*fluxcdv1.ResourceSetInputProvider)
		newObj := e.ObjectNew.(*fluxcdv1.ResourceSetInputProvider)

		// Trigger reconciliation only if the exported inputs have changed.
		return oldObj.Status.LastExportedRevision != newObj.Status.LastExportedRevision
	},
}
