// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetInputProviderReconcilerOptions contains options for the reconciler.
type ResourceSetInputProviderReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetInputProviderReconciler) SetupWithManager(mgr ctrl.Manager, opts ResourceSetInputProviderReconcilerOptions) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1.ResourceSetInputProvider{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
				),
			))

	// Establish a non-blocking raw source watch on ExternalArtifact.
	// Since ExternalArtifact CRD might not exist at startup, we poll the cache
	// every 10s until GetInformer returns successfully, then register the watch event handlers.
	b = b.WatchesRawSource(source.TypedFunc[reconcile.Request](
		func(ctx context.Context, q workqueue.TypedRateLimitingInterface[reconcile.Request]) error {
			go func() {
				u := &unstructured.Unstructured{}
				u.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   "source.toolkit.fluxcd.io",
					Version: "v1",
					Kind:    "ExternalArtifact",
				})
				_ = wait.PollUntilContextCancel(ctx, 10*time.Second, true, func(ctx context.Context) (bool, error) {
					inf, err := mgr.GetCache().GetInformer(ctx, u, ctrlcache.BlockUntilSynced(false))
					if err != nil {
						// CRD not installed yet; retry without blocking controller startup.
						return false, nil
					}
					_, err = inf.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
						AddFunc: func(obj any) {
							r.enqueueExternalArtifact(ctx, q, obj)
						},
						DeleteFunc: func(obj any) {
							r.enqueueExternalArtifact(ctx, q, obj)
						},
						UpdateFunc: func(oldObj, newObj any) {
							r.enqueueExternalArtifact(ctx, q, oldObj) // catches label changes away from selector
							r.enqueueExternalArtifact(ctx, q, newObj)
						},
					})
					return err == nil, err
				})
			}()
			return nil
		},
	))

	// Watch Namespace changes to enqueue ResourceSetInputProviders that select
	// namespaces by labels through spec.selectors[].namespaceSelector.
	b = b.Watches(
		&corev1.Namespace{},
		handler.Funcs{
			CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				r.enqueueNamespace(ctx, q, e.Object)
			},
			UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				r.enqueueNamespace(ctx, q, e.ObjectOld) // catches label changes away from selector
				r.enqueueNamespace(ctx, q, e.ObjectNew)
			},
			DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
				r.enqueueNamespace(ctx, q, e.Object)
			},
		},
	)

	return b.WithOptions(controller.Options{
		RateLimiter: opts.RateLimiter,
	}).Complete(r)
}

// enqueueExternalArtifact handles DeletedFinalStateUnknown tombstones and enqueues mapped ResourceSetInputProviders.
func (r *ResourceSetInputProviderReconciler) enqueueExternalArtifact(
	ctx context.Context,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
	obj any,
) {
	if d, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}
	if o, ok := obj.(client.Object); ok {
		for _, req := range r.requestsForExternalArtifacts(ctx, o) {
			q.Add(req)
		}
	}
}

// enqueueNamespace handles DeletedFinalStateUnknown tombstones and enqueues
// mapped ResourceSetInputProviders.
func (r *ResourceSetInputProviderReconciler) enqueueNamespace(
	ctx context.Context,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
	obj any,
) {
	if d, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}
	if o, ok := obj.(client.Object); ok {
		for _, req := range r.requestsForNamespace(ctx, o) {
			q.Add(req)
		}
	}
}

// requestsForNamespace maps a Namespace change event to reconcile requests
// for ResourceSetInputProviders whose namespaceSelector matches the Namespace.
func (r *ResourceSetInputProviderReconciler) requestsForNamespace(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	var list fluxcdv1.ResourceSetInputProviderList
	listOpts := []client.ListOption{
		client.UnsafeDisableDeepCopy,
	}
	if err := r.List(ctx, &list, listOpts...); err != nil {
		log.Error(err, "failed to list ResourceSetInputProviders for Namespace change")
		return nil
	}

	objLabels := labels.Set(obj.GetLabels())

	var reqs []reconcile.Request
	for _, rsip := range list.Items {
		if rsip.Spec.Type != fluxcdv1.InputProviderExternalArtifact {
			continue
		}

		matched := false
		for _, sel := range rsip.Spec.Selectors {
			if sel.NamespaceSelector == nil {
				continue
			}
			selector, err := metav1.LabelSelectorAsSelector(sel.NamespaceSelector)
			if err != nil {
				log.Error(err, "failed to convert namespace selector from ResourceSetInputProvider spec.selectors",
					"name", rsip.Name, "namespace", rsip.Namespace)
				continue
			}
			if selector.Matches(objLabels) {
				matched = true
				break
			}
		}

		if matched {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      rsip.Name,
					Namespace: rsip.Namespace,
				},
			})
		}
	}
	return reqs
}

// requestsForExternalArtifacts maps an ExternalArtifact change event to reconcile requests for
// ResourceSetInputProviders whose spec.selectors matches the ExternalArtifact.
func (r *ResourceSetInputProviderReconciler) requestsForExternalArtifacts(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	// List all ResourceSetInputProviders across all namespaces since we now support cross-namespace selectors.
	var list fluxcdv1.ResourceSetInputProviderList
	listOpts := []client.ListOption{
		client.UnsafeDisableDeepCopy,
	}
	if err := r.List(ctx, &list, listOpts...); err != nil {
		log.Error(err, "failed to list ResourceSetInputProviders for ExternalArtifact change")
		return nil
	}

	objLabels := labels.Set(obj.GetLabels())

	var reqs []reconcile.Request
	for _, rsip := range list.Items {
		if rsip.Spec.Type != fluxcdv1.InputProviderExternalArtifact {
			continue
		}
		if len(rsip.Spec.Selectors) == 0 {
			continue
		}

		matched := false
		for _, sel := range rsip.Spec.Selectors {
			// Namespace boundary check
			if !r.externalArtifactSelectorMatchesNamespace(ctx, rsip.GetNamespace(), sel, obj.GetNamespace()) {
				continue
			}

			if sel.Name != "" {
				if sel.Name == obj.GetName() {
					matched = true
					break
				}
			} else {
				selector, err := metav1.LabelSelectorAsSelector(&sel.LabelSelector)
				if err != nil {
					log.Error(err, "failed to convert label selector from ResourceSetInputProvider spec.selectors to selector",
						"name", rsip.Name,
						"namespace", rsip.Namespace)
					continue
				}
				if selector.Matches(objLabels) {
					matched = true
					break
				}
			}
		}

		if matched {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      rsip.Name,
					Namespace: rsip.Namespace,
				},
			})
		}
	}
	return reqs
}

// externalArtifactSelectorMatchesNamespace returns true when the given
// ExternalArtifact namespace satisfies the namespace scope of the selector.
// NamespaceSelector is evaluated against the in-cluster Namespace labels,
// while an explicit Namespace is matched by name.
func (r *ResourceSetInputProviderReconciler) externalArtifactSelectorMatchesNamespace(
	ctx context.Context,
	providerNamespace string,
	sel fluxcdv1.ExternalArtifactSelector,
	namespace string,
) bool {
	if sel.NamespaceSelector == nil {
		ns := sel.Namespace
		if ns == "" {
			ns = providerNamespace
		}
		return ns == "*" || ns == namespace
	}

	selector, err := metav1.LabelSelectorAsSelector(sel.NamespaceSelector)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to convert namespace selector from ResourceSetInputProvider spec.selectors")
		return false
	}

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		return false
	}
	return selector.Matches(labels.Set(ns.GetLabels()))
}
