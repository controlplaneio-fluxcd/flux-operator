// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// EventWatcher process Flux events.
type EventWatcher struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme *runtime.Scheme
}

// Reconcile processes a Flux event.
func (r *EventWatcher) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &corev1.Event{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	msg := fmt.Sprintf("Received %s event from %s/%s/%s: %s",
		obj.Type,
		obj.InvolvedObject.Kind,
		obj.InvolvedObject.Namespace,
		obj.InvolvedObject.Name,
		obj.Message)
	log.Info(msg)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventWatcher) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}, builder.WithPredicates(FluxEventsPredicate{setupLog: mgr.GetLogger()})).
		Complete(r)
}

// FluxEventsPredicate filters Flux events.
type FluxEventsPredicate struct {
	predicate.Funcs
	setupLog logr.Logger
}

func (r FluxEventsPredicate) Create(e event.CreateEvent) bool {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(e.Object)
	if err != nil {
		r.setupLog.Error(err, "Event watcher failed to convert the received event")
		return false
	}

	inKind, _, err := unstructured.NestedString(obj, "involvedObject", "kind")
	if err != nil {
		r.setupLog.Error(err, "Event watcher failed to extract the involved object kind")
		return false
	}

	// Only watch events about cluster state changes.
	if !slices.Contains([]string{"Kustomization", "HelmRelease"}, inKind) {
		return false
	}

	// Ignore events older than a minute.
	if lastTS, found, err := unstructured.NestedString(obj, "lastTimestamp"); err == nil && found {
		var ts metav1.Time
		if err := ts.UnmarshalText([]byte(lastTS)); err == nil {
			if time.Since(ts.Time) < time.Minute {
				return true
			}
		}
	}

	return false
}

func (r FluxEventsPredicate) Update(e event.UpdateEvent) bool {
	return false
}

func (r FluxEventsPredicate) Delete(e event.DeleteEvent) bool {
	return false
}
