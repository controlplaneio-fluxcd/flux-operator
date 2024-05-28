// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	fluxcdv1alpha1 "github.com/controlplaneio-fluxcd/fluxcd-operator/api/v1alpha1"
	"github.com/controlplaneio-fluxcd/fluxcd-operator/internal/builder"
)

// FluxInstanceReconciler reconciles a FluxInstance object
type FluxInstanceReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme        *runtime.Scheme
	StatusPoller  *polling.StatusPoller
	StatusManager string
	StoragePath   string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=fluxinstances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *FluxInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &fluxcdv1alpha1.FluxInstance{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize the runtime patcher with the current version of the object.
	patcher := patch.NewSerialPatcher(obj, r.Client)

	// Finalise the reconciliation and report the results.
	defer func() {
		if err := r.finalizeStatus(ctx, obj, patcher); err != nil {
			log.Error(err, "failed to update status")
			retErr = kerrors.NewAggregate([]error{retErr, err})
		}
	}()

	// Uninstall if the object is under deletion.
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, obj)
	}

	// Add the finalizer if it does not exist.
	if !controllerutil.ContainsFinalizer(obj, fluxcdv1alpha1.Finalizer) {
		log.Info("Adding finalizer", "finalizer", fluxcdv1alpha1.Finalizer)
		controllerutil.AddFinalizer(obj, fluxcdv1alpha1.Finalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *FluxInstanceReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	// Mark the object as reconciling.
	msg := "Reconciliation in progress"
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		msg)
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		msg)

	// Build the distribution manifests.
	buildResult, err := r.build(ctx, obj)
	if err != nil {
		msg := fmt.Sprintf("build failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.BuildFailedReason,
			msg)
		conditions.MarkTrue(obj,
			meta.StalledCondition,
			meta.BuildFailedReason,
			msg)
		log.Error(err, msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.BuildFailedReason, msg)
		return ctrl.Result{}, nil
	}

	// Update latest attempted revision.
	if obj.Status.LastAttemptedRevision != buildResult.Revision {
		msg := fmt.Sprintf("Upagrading to revision %s", buildResult.Revision)
		if obj.Status.LastAttemptedRevision == "" {
			msg = fmt.Sprintf("Installing revision %s", buildResult.Revision)
		}
		log.Info(msg)
		r.EventRecorder.Event(obj, corev1.EventTypeNormal, meta.ProgressingReason, msg)
		obj.Status.LastAttemptedRevision = buildResult.Revision
	}
	if err := r.patch(ctx, obj, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	// Mark the object as ready.
	msg = fmt.Sprintf("Reconciliation finished in %s", time.Since(reconcileStart).String())
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		msg)
	log.Info(msg, "revision", obj.Status.LastAttemptedRevision)
	r.EventRecorder.AnnotatedEventf(obj,
		map[string]string{fluxcdv1alpha1.RevisionAnnotation: obj.Status.LastAttemptedRevision},
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	// Requeue the reconciliation if the interval is set in annotations.
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result, nil
}

func (r *FluxInstanceReconciler) build(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance) (*builder.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	fluxDir := filepath.Join(r.StoragePath, "flux")
	if _, err := os.Stat(fluxDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("storage path %s does not exist", fluxDir)
	}

	ver, err := builder.MatchVersion(fluxDir, obj.Spec.Distribution.Version)
	if err != nil {
		return nil, err
	}

	options := builder.MakeDefaultOptions()
	options.Version = ver
	options.Registry = obj.Spec.Distribution.Registry
	options.ImagePullSecret = obj.Spec.Distribution.ImagePullSecret
	options.Namespace = obj.GetNamespace()

	if obj.Spec.Kustomize != nil && len(obj.Spec.Kustomize.Patches) > 0 {
		patchesData, err := yaml.Marshal(obj.Spec.Kustomize.Patches)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kustomize patches: %w", err)
		}
		options.Patches = string(patchesData)
	}

	tmpDir, err := builder.MkdirTempAbs("", "flux")
	if err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Error(err, "failed to remove tmp dir", "dir", tmpDir)
		}
	}()

	return builder.Build(filepath.Join(fluxDir, ver), tmpDir, options)
}

//nolint:unparam
func (r *FluxInstanceReconciler) finalize(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance) (ctrl.Result, error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	controllerutil.RemoveFinalizer(obj, fluxcdv1alpha1.Finalizer)

	msg := fmt.Sprintf("Uninstallation completed in %v", time.Since(reconcileStart).String())
	log.Info(msg)

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

func (r *FluxInstanceReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
	patcher *patch.SerialPatcher) error {
	// Set the value of the reconciliation request in status.
	if v, ok := meta.ReconcileAnnotationValue(obj.GetAnnotations()); ok {
		obj.Status.LastHandledReconcileAt = v
	}

	// Set the Reconciling reason to ProgressingWithRetry if the
	// reconciliation has failed.
	if conditions.IsFalse(obj, meta.ReadyCondition) &&
		conditions.Has(obj, meta.ReconcilingCondition) {
		rc := conditions.Get(obj, meta.ReconcilingCondition)
		rc.Reason = meta.ProgressingWithRetryReason
		conditions.Set(obj, rc)
	}

	// Remove the Reconciling condition.
	if conditions.IsTrue(obj, meta.ReadyCondition) || conditions.IsTrue(obj, meta.StalledCondition) {
		conditions.Delete(obj, meta.ReconcilingCondition)
	}

	// Patch finalizers, status and conditions.
	return r.patch(ctx, obj, patcher)
}

func (r *FluxInstanceReconciler) patch(ctx context.Context,
	obj *fluxcdv1alpha1.FluxInstance,
	patcher *patch.SerialPatcher) (retErr error) {
	// Configure the runtime patcher.
	ownedConditions := []string{
		meta.ReadyCondition,
		meta.ReconcilingCondition,
		meta.StalledCondition,
	}
	patchOpts := []patch.Option{
		patch.WithOwnedConditions{Conditions: ownedConditions},
		patch.WithForceOverwriteConditions{},
		patch.WithFieldOwner(r.StatusManager),
	}

	if conditions.IsTrue(obj, meta.ReadyCondition) || conditions.IsTrue(obj, meta.StalledCondition) {
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
	}

	// Patch the object status, conditions and finalizers.
	if err := patcher.Patch(ctx, obj, patchOpts...); err != nil {
		if !obj.GetDeletionTimestamp().IsZero() {
			err = kerrors.FilterOut(err, func(e error) bool { return apierrors.IsNotFound(e) })
		}
		retErr = kerrors.NewAggregate([]error{retErr, err})
		if retErr != nil {
			return retErr
		}
	}

	return nil
}
