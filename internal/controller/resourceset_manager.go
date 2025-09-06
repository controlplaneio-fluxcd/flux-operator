// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
	RateLimiter           workqueue.TypedRateLimiter[reconcile.Request]
	WatchConfigsPredicate predicate.Predicate
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
			handler.EnqueueRequestsFromMapFunc(r.requestsForResourceSetInputProviders),
			builder.WithPredicates(exportedInputsChangePredicate),
		).
		WatchesMetadata(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForConfigMapsOrSecrets),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, opts.WatchConfigsPredicate),
		).
		WatchesMetadata(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.requestsForConfigMapsOrSecrets),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, opts.WatchConfigsPredicate),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(r)
}

// requestsForResourceSetInputProviders lists all ResourceSets in the same namespace as the
// object that triggered the event and returns a list of reconcile.Requests
// for those ResourceSets that have an InputsFrom field that matches the
// object that triggered the event. It works for any object type that
// implements the client.Object interface.
func (r *ResourceSetReconciler) requestsForResourceSetInputProviders(
	ctx context.Context, clientObject client.Object) []reconcile.Request {

	log := ctrl.LoggerFrom(ctx)
	obj := clientObject.(*fluxcdv1.ResourceSetInputProvider)

	// Compute object metadata.
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
	for _, rset := range list.Items {

		var matches bool

		// Check if it least one item in InputsFrom matches the object.
		for i, inputsFrom := range rset.Spec.InputsFrom {

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

// requestsForConfigMapsOrSecrets lists the metadata of all ConfigMaps or Secrets created
// by ResourceSets (depending on the kind of the object that triggered the event), and for
// each ResourceSet being referenced in a copyFrom annotation, a reconcile.Request is
// included in the returned slice.
func (r *ResourceSetReconciler) requestsForConfigMapsOrSecrets(ctx context.Context,
	obj client.Object) []reconcile.Request {

	log := ctrl.LoggerFrom(ctx)

	// Compute object metadata.
	objKind := obj.GetObjectKind().GroupVersionKind().Kind
	objKey := client.ObjectKeyFromObject(obj).String()

	// List the metadata of all objects of the same kind that were created by
	// ResourceSets. The WatchesMetadata call in SetupWithManager causes
	// controller-runtime to cache the metadata of the object type, even if
	// caching for the concrete type is disabled (which is the case for
	// ConfigMaps and Secrets). The documentation states that controllers
	// fetching such watched objects should use the
	// metav1.PartialObjectMetadataList type to avoid duplicating the cache:
	//
	// https://github.com/kubernetes-sigs/controller-runtime/blob/98b5b2285d1b38eff457e126e3fcb41908dc7606/pkg/builder/controller.go#L177-L203
	//
	// Hence the List operation will hit the cache, and not API server.
	var appliedObjects metav1.PartialObjectMetadataList
	appliedObjects.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(objKind))
	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: resourceSetOwnerSelector,
		},
		// We will be listing potentially a large number of objects, so we
		// disable deep copy to avoid unnecessary memory allocations. It's
		// safe to do so because we are not modifying the objects here.
		client.UnsafeDisableDeepCopy,
	}
	if err := r.List(ctx, &appliedObjects, listOpts...); err != nil {
		log.Error(err, "failed to list config objects applied by ResourceSets",
			"eventTrigger", map[string]any{
				"kind":      objKind,
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			})
		return nil
	}

	// Match listed objects with the object that triggered the event
	// to generate a list of reconcile.Requests.
	resourceSets := make(map[types.NamespacedName]struct{})
	for _, appliedObject := range appliedObjects.Items {
		copyFrom := appliedObject.GetAnnotations()[fluxcdv1.CopyFromAnnotation]
		if copyFrom != objKey {
			continue
		}
		objLabels := appliedObject.GetLabels()
		rset := types.NamespacedName{
			Name:      objLabels[fluxcdv1.OwnerLabelResourceSetName],
			Namespace: objLabels[fluxcdv1.OwnerLabelResourceSetNamespace],
		}
		resourceSets[rset] = struct{}{}
	}
	reqs := make([]reconcile.Request, 0, len(resourceSets))
	for rset := range resourceSets {
		reqs = append(reqs, reconcile.Request{NamespacedName: rset})
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

var resourceSetOwnerSelector = func() labels.Selector {
	name, err := labels.NewRequirement(fluxcdv1.OwnerLabelResourceSetName, selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	namespace, err := labels.NewRequirement(fluxcdv1.OwnerLabelResourceSetNamespace, selection.Exists, nil)
	if err != nil {
		panic(err)
	}
	return labels.NewSelector().Add(*name, *namespace)
}()
