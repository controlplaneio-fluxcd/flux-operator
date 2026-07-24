// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"time"

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
			ns := sel.Namespace
			if ns == "" {
				ns = rsip.GetNamespace()
			}

			// Namespace boundary check
			if ns != "*" && ns != obj.GetNamespace() {
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
