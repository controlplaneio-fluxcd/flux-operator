// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"strings"

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

	runtimectrl "github.com/fluxcd/pkg/runtime/controller"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetReconcilerOptions contains options for the reconciler.
type ResourceSetReconcilerOptions struct {
	RateLimiter             workqueue.TypedRateLimiter[reconcile.Request]
	WatchConfigsPredicate   predicate.Predicate
	DisableWaitInterruption bool
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, opts ResourceSetReconcilerOptions) error {
	var blder *builder.Builder
	var toComplete reconcile.TypedReconciler[reconcile.Request]
	var enqueueRequestsFromMapFunc func(objKind string, fn handler.MapFunc) handler.EventHandler

	rsetPredicate := predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)

	if opts.DisableWaitInterruption {
		toComplete = r
		enqueueRequestsFromMapFunc = func(objKind string, fn handler.MapFunc) handler.EventHandler {
			return handler.EnqueueRequestsFromMapFunc(fn)
		}
		blder = ctrl.NewControllerManagedBy(mgr).
			For(&fluxcdv1.ResourceSet{}, builder.WithPredicates(rsetPredicate))
	} else {
		wr := runtimectrl.WrapReconciler(r)
		toComplete = wr
		enqueueRequestsFromMapFunc = wr.EnqueueRequestsFromMapFunc
		blder = runtimectrl.NewControllerManagedBy(mgr, wr).
			For(&fluxcdv1.ResourceSet{}, rsetPredicate).Builder
	}

	return blder.
		Watches(
			&fluxcdv1.ResourceSetInputProvider{},
			enqueueRequestsFromMapFunc(fluxcdv1.ResourceSetInputProviderKind, r.requestsForResourceSetInputProviders),
			builder.WithPredicates(exportedInputsChangePredicate),
		).
		WatchesMetadata(
			&corev1.ConfigMap{},
			enqueueRequestsFromMapFunc("ConfigMap", r.requestsForConfigMapsOrSecrets),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, opts.WatchConfigsPredicate),
		).
		WatchesMetadata(
			&corev1.Secret{},
			enqueueRequestsFromMapFunc("Secret", r.requestsForConfigMapsOrSecrets),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}, opts.WatchConfigsPredicate),
		).
		WithOptions(controller.Options{
			RateLimiter: opts.RateLimiter,
		}).Complete(toComplete)
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

// requestsForConfigMapsOrSecrets enqueues ResourceSets that depend on the
// ConfigMap or Secret that triggered the event. A ResourceSet is
// considered dependent if either:
//   - it applied a ConfigMap or Secret that uses the triggering object
//     as a copyFrom source, or
//   - it applied a ConfigMap that uses the triggering Secret as a
//     convertKubeConfigFrom source, or
//   - its status.externalChecksumRefs lists the triggering object.
func (r *ResourceSetReconciler) requestsForConfigMapsOrSecrets(ctx context.Context,
	obj client.Object) []reconcile.Request {

	log := ctrl.LoggerFrom(ctx)

	// Compute object metadata.
	objKind := obj.GetObjectKind().GroupVersionKind().Kind
	objKey := client.ObjectKeyFromObject(obj).String()
	objRef := objKind + "/" + objKey

	resourceSets := make(map[types.NamespacedName]struct{})

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
	// via the copyFrom annotation.
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

	// When the trigger is a Secret, match applied ConfigMaps whose
	// convertKubeConfigFrom annotation references it. The annotation
	// value may be 'namespace/name' or 'namespace/name:key'.
	if objKind == kindSecret {
		var appliedConfigMaps metav1.PartialObjectMetadataList
		appliedConfigMaps.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(kindConfigMap))
		if err := r.List(ctx, &appliedConfigMaps, listOpts...); err != nil {
			log.Error(err, "failed to list ConfigMaps applied by ResourceSets",
				"eventTrigger", map[string]any{
					"kind":      objKind,
					"name":      obj.GetName(),
					"namespace": obj.GetNamespace(),
				})
		} else {
			for _, appliedObject := range appliedConfigMaps.Items {
				convertFrom := appliedObject.GetAnnotations()[fluxcdv1.ConvertKubeConfigFromAnnotation]
				if convertFrom == "" {
					continue
				}
				nameRef := convertFrom
				if colonIdx := strings.LastIndex(convertFrom, ":"); colonIdx > 0 {
					if slashIdx := strings.Index(convertFrom, "/"); slashIdx > 0 && colonIdx > slashIdx {
						nameRef = convertFrom[:colonIdx]
					}
				}
				if nameRef != objKey {
					continue
				}
				objLabels := appliedObject.GetLabels()
				rset := types.NamespacedName{
					Name:      objLabels[fluxcdv1.OwnerLabelResourceSetName],
					Namespace: objLabels[fluxcdv1.OwnerLabelResourceSetNamespace],
				}
				resourceSets[rset] = struct{}{}
			}
		}
	}

	// Match ResourceSets whose status.externalChecksumRefs contains the
	// triggering object. The ResourceSet informer is a full-object cache,
	// so this List hits memory rather than the API server.
	var rsetList fluxcdv1.ResourceSetList
	if err := r.List(ctx, &rsetList, client.UnsafeDisableDeepCopy); err != nil {
		log.Error(err, "failed to list ResourceSets for checksum ref match",
			"eventTrigger", map[string]any{
				"kind":      objKind,
				"name":      obj.GetName(),
				"namespace": obj.GetNamespace(),
			})
	} else {
		for _, rs := range rsetList.Items {
			for _, ref := range rs.Status.ExternalChecksumRefs {
				if ref == objRef {
					resourceSets[types.NamespacedName{Name: rs.Name, Namespace: rs.Namespace}] = struct{}{}
					break
				}
			}
		}
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
