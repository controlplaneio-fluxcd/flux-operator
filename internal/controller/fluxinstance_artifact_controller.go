// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
)

// FluxInstanceArtifactReconciler reconciles the distribution artifact of a FluxInstance object
type FluxInstanceArtifactReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	StatusManager string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FluxInstanceArtifactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	obj := &fluxcdv1.FluxInstance{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip reconciliation if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		return ctrl.Result{}, nil
	}

	// Skip reconciliation if the object does not have a last artifact revision to avoid race condition.
	if obj.Status.LastArtifactRevision == "" {
		return requeueArtifactAfter(obj), nil
	}

	// Skip reconciliation if the object is not ready.
	if !conditions.IsReady(obj) {
		return requeueArtifactAfter(obj), nil
	}

	// Reconcile the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)
	return r.reconcile(ctx, obj, patcher)
}

func (r *FluxInstanceArtifactReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {

	log := ctrl.LoggerFrom(ctx)

	// Fetch the latest digest of the distribution manifests.
	artifactURL := obj.Spec.Distribution.Artifact
	artifactDigest, err := builder.HeadArtifact(ctx, artifactURL)
	if err != nil {
		msg := fmt.Sprintf("fetch failed: %s", err.Error())
		r.Event(obj, corev1.EventTypeWarning, meta.ArtifactFailedReason, msg)
		return ctrl.Result{}, err
	}
	log.V(1).Info("fetched latest manifests", "url", artifactURL, "digest", artifactDigest)

	// Skip reconciliation if the artifact has not changed.
	if artifactDigest == obj.Status.LastArtifactRevision {
		return requeueArtifactAfter(obj), nil
	}

	// The digest has changed, request a reconciliation.
	log.Info("artifact revision changed, requesting a reconciliation",
		"old", obj.Status.LastArtifactRevision, "new", artifactDigest)
	if obj.Annotations == nil {
		obj.Annotations = make(map[string]string, 1)
	}
	obj.Annotations[meta.ReconcileRequestAnnotation] = time.Now().Format(time.RFC3339Nano)
	if err := patcher.Patch(ctx, obj, patch.WithFieldOwner(r.StatusManager)); err != nil {
		return ctrl.Result{}, err
	}

	return requeueArtifactAfter(obj), nil
}

// requeueArtifactAfter returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotation for artifact reconciliation.
func requeueArtifactAfter(obj *fluxcdv1.FluxInstance) ctrl.Result {
	result := ctrl.Result{}
	if d := obj.GetArtifactInterval(); d > 0 {
		result.RequeueAfter = d
	}
	return result
}
