// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourceSetInputProviderReconcilerOptions contains options for the reconciler.
type ResourceSetInputProviderReconcilerOptions struct {
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceSetInputProviderReconciler) SetupWithManager(mgr ctrl.Manager, opts ResourceSetInputProviderReconcilerOptions) error {
	log := ctrl.LoggerFrom(context.Background()).WithName("ResourceSetInputProvider")

	b := ctrl.NewControllerManagedBy(mgr).
		For(&fluxcdv1.ResourceSetInputProvider{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{},
					predicate.AnnotationChangedPredicate{},
				),
			))

	// Check if the ExternalArtifact CRD is installed in the cluster.
	// If it is, register a watch on it.
	gvk := schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta1",
		Kind:    "ExternalArtifact",
	}
	if _, err := mgr.GetRESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err == nil {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		b = b.Watches(
			u,
			handler.EnqueueRequestsFromMapFunc(r.requestsForExternalArtifacts),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		)
		log.Info("Registered watch for ExternalArtifact resources")
	} else {
		log.Info("ExternalArtifact CRD not found in cluster, in-cluster artifact watches are disabled")
	}

	return b.WithOptions(controller.Options{
		RateLimiter: opts.RateLimiter,
	}).Complete(r)
}

// requestsForExternalArtifacts maps an ExternalArtifact change event to reconcile requests for
// ResourceSetInputProviders in the same namespace whose label selector matches the ExternalArtifact's labels.
func (r *ResourceSetInputProviderReconciler) requestsForExternalArtifacts(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	log := ctrl.LoggerFrom(ctx)

	// List all ResourceSetInputProviders in the same namespace as the ExternalArtifact.
	var list fluxcdv1.ResourceSetInputProviderList
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
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
		if rsip.Spec.Selector == nil {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(rsip.Spec.Selector)
		if err != nil {
			log.Error(err, "failed to convert label selector from ResourceSetInputProvider spec.selector to selector",
				"name", rsip.Name,
				"namespace", rsip.Namespace)
			continue
		}
		if selector.Matches(objLabels) {
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
