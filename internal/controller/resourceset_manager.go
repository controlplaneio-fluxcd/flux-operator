// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
			handler.EnqueueRequestsFromMapFunc(r.requestsForChangeOf),
			builder.WithPredicates(exportedInputsChangePredicate),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(r)
}

// requestsForChangeOf lists all ResourceSets in the same namespace as the
// object that triggered the event and returns a list of reconcile.Requests
// for those ResourceSets that have an InputsFrom field that matches the
// object that triggered the event. It works for any object type that
// implements the client.Object interface.
func (r *ResourceSetReconciler) requestsForChangeOf(ctx context.Context, obj client.Object) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	// Compute object metadata.
	var objAPIVersion, objKind string
	switch obj.(type) {
	case *fluxcdv1.ResourceSetInputProvider:
		objAPIVersion = fluxcdv1.GroupVersion.String()
		objKind = fluxcdv1.ResourceSetInputProviderKind
	default:
		return nil
	}
	objName := obj.GetName()
	objLabels := labels.Set(obj.GetLabels())

	// List all ResourceSets in the same namespace as the object that
	// triggered the event.
	var list fluxcdv1.ResourceSetList
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
		// We will be listing potentially a large number of objects, so we
		// disable deep copy to avoid unnecessary memory allocations. It's
		// safe to do so because we are not modifying the objects here.
		client.UnsafeDisableDeepCopy,
	}
	if err := r.List(ctx, &list, listOpts...); err != nil {
		log.Error(err, "failed to list objects for provider change")
		return nil
	}

	// Match listed ResourceSets with the object that triggered the event
	// to generate a list of reconcile.Requests.
	var reqs []reconcile.Request
	inputsFromDefaultAPIVersion := fluxcdv1.GroupVersion.String()
	for _, rset := range list.Items {

		var matches bool

		// Check if it least one item in InputsFrom matches the object.
		for i, inputsFrom := range rset.Spec.InputsFrom {

			// Skip if API version doesn't match.
			apiVersion := inputsFrom.APIVersion
			if apiVersion == "" {
				apiVersion = inputsFromDefaultAPIVersion
			}
			if apiVersion != objAPIVersion {
				continue
			}

			// Skip if kind doesn't match.
			if inputsFrom.Kind != objKind {
				continue
			}

			// Skip if name doesn't match.
			if name := inputsFrom.Name; name != "" {
				if name == objName {
					matches = true
					break
				}
				continue
			}

			// Skip if ls doesn't match.
			if ls := inputsFrom.Selector; ls != nil {
				selector, err := metav1.LabelSelectorAsSelector(ls)
				if err != nil {
					log.Error(err, "failed to convert label selector from ResourceSet spec.inputsFrom to selector",
						"resourceSet", map[string]any{
							"name":      rset.Name,
							"namespace": rset.Namespace,
							"inputsFrom": map[string]any{
								"index": i,
								"spec":  inputsFrom,
							},
						})
					continue
				}
				if selector.Matches(objLabels) {
					matches = true
					break
				}
				continue
			}
		}

		// Enqueue the request if we have a match.
		if matches {
			key := types.NamespacedName{Name: rset.Name, Namespace: rset.Namespace}
			reqs = append(reqs, reconcile.Request{NamespacedName: key})
		}
	}

	return reqs
}

var exportedInputsChangePredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldObj := e.ObjectOld.(*fluxcdv1.ResourceSetInputProvider)
		newObj := e.ObjectNew.(*fluxcdv1.ResourceSetInputProvider)

		// Trigger reconciliation only if the exported inputs have changed.
		return oldObj.Status.LastExportedRevision != newObj.Status.LastExportedRevision
	},
}
