// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

// ResourceGroupReconciler reconciles a ResourceGroup object
type ResourceGroupReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	APIReader     client.Reader
	Scheme        *runtime.Scheme
	StatusPoller  *polling.StatusPoller
	StatusManager string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcegroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcegroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcegroups/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &fluxcdv1.ResourceGroup{}
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
		return r.uninstall(ctx, obj)
	}

	// Add the finalizer if it does not exist.
	if !controllerutil.ContainsFinalizer(obj, fluxcdv1.Finalizer) {
		log.Info("Adding finalizer", "finalizer", fluxcdv1.Finalizer)
		controllerutil.AddFinalizer(obj, fluxcdv1.Finalizer)
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			"%s", msgInProgress)
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			"%s", msgInProgress)
		return ctrl.Result{Requeue: true}, nil
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		msg := "Reconciliation in disabled"
		log.Error(errors.New("can't reconcile instance"), msg)
		r.Event(obj, corev1.EventTypeWarning, "ReconciliationDisabled", msg)
		return ctrl.Result{}, nil
	}

	// Check dependencies and requeue the reconciliation if the check fails.
	if err := r.checkDependencies(ctx, obj); err != nil {
		msg := fmt.Sprintf("Retrying dependency check: %s", err.Error())
		if conditions.GetReason(obj, meta.ReadyCondition) != meta.DependencyNotReadyReason {
			log.Error(err, "dependency check failed")
			r.Event(obj, corev1.EventTypeNormal, meta.DependencyNotReadyReason, msg)
		}
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.DependencyNotReadyReason,
			"%s", msg)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *ResourceGroupReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.ResourceGroup,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	// Mark the object as reconciling.
	msg := "Reconciliation in progress"
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		"%s", msg)
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		"%s", msg)
	if err := r.patch(ctx, obj, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	// Build the resources.
	buildResult, err := builder.BuildResourceGroup(obj.Spec.Resources, obj.GetInputs())
	if err != nil {
		msg := fmt.Sprintf("build failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.BuildFailedReason,
			"%s", msg)
		conditions.MarkTrue(obj,
			meta.StalledCondition,
			meta.BuildFailedReason,
			"%s", msg)
		log.Error(err, msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.BuildFailedReason, msg)
		return ctrl.Result{}, nil
	}

	// Apply the resources to the cluster.
	if err := r.apply(ctx, obj, buildResult); err != nil {
		msg := fmt.Sprintf("reconciliation failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			"%s", msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)

		return ctrl.Result{}, err
	}

	// Mark the object as ready.
	msg = fmt.Sprintf("Reconciliation finished in %s", fmtDuration(reconcileStart))
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfterResourceGroup(obj), nil
}

func (r *ResourceGroupReconciler) checkDependencies(ctx context.Context,
	obj *fluxcdv1.ResourceGroup) error {

	for _, dep := range obj.Spec.DependsOn {
		depObj := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": dep.APIVersion,
				"kind":       dep.Kind,
				"metadata": map[string]interface{}{
					"name":      dep.Name,
					"namespace": dep.Namespace,
				},
			},
		}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(depObj), depObj); err != nil {
			return fmt.Errorf("dependency %s/%s/%s not found: %w", dep.APIVersion, dep.Kind, dep.Name, err)
		}

		if dep.Ready {
			stat, err := status.Compute(depObj)
			if err != nil {
				return fmt.Errorf("dependency %s/%s/%s not ready: %w", dep.APIVersion, dep.Kind, dep.Name, err)
			}

			if stat.Status != status.CurrentStatus {
				return fmt.Errorf("dependency %s/%s/%s not ready: status %s", dep.APIVersion, dep.Kind, dep.Name, stat.Status)
			}
		}
	}

	return nil
}

// apply reconciles the resources in the cluster by performing
// a server-side apply, pruning of stale resources and waiting
// for the resources to become ready.
func (r *ResourceGroupReconciler) apply(ctx context.Context,
	obj *fluxcdv1.ResourceGroup,
	objects []*unstructured.Unstructured) error {
	log := ctrl.LoggerFrom(ctx)
	var changeSetLog strings.Builder

	// Create a snapshot of the current inventory.
	oldInventory := inventory.New()
	if obj.Status.Inventory != nil {
		obj.Status.Inventory.DeepCopyInto(oldInventory)
	}

	// Create a resource manager to reconcile the resources.
	resourceManager := ssa.NewResourceManager(r.Client, r.StatusPoller, ssa.Owner{
		Field: r.StatusManager,
		Group: fmt.Sprintf("resourcegroup.%s", fluxcdv1.GroupVersion.Group),
	})
	resourceManager.SetOwnerLabels(objects, obj.GetName(), obj.GetNamespace())

	if err := normalize.UnstructuredList(objects); err != nil {
		return err
	}

	if cm := obj.Spec.CommonMetadata; cm != nil {
		ssautil.SetCommonMetadata(objects, cm.Labels, cm.Annotations)
	}

	applyOpts := ssa.DefaultApplyOptions()
	applyOpts.Cleanup = ssa.ApplyCleanupOptions{
		// Remove the kubectl and helm annotations.
		Annotations: []string{
			corev1.LastAppliedConfigAnnotation,
			"meta.helm.sh/release-name",
			"meta.helm.sh/release-namespace",
		},
		// Remove the flux labels set at bootstrap.
		Labels: []string{
			"kustomize.toolkit.fluxcd.io/name",
			"kustomize.toolkit.fluxcd.io/namespace",
		},
		// Take ownership of the Flux resources if they
		// were previously managed by other tools.
		FieldManagers: []ssa.FieldManager{
			{
				Name:          "flux",
				OperationType: metav1.ManagedFieldsOperationApply,
			},
			{
				Name:          "kustomize-controller",
				OperationType: metav1.ManagedFieldsOperationApply,
			},
			{
				Name:          "helm",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
			{
				Name:          "kubectl",
				OperationType: metav1.ManagedFieldsOperationUpdate,
			},
		},
	}

	resultSet := ssa.NewChangeSet()

	// Apply the resources to the cluster.
	changeSet, err := resourceManager.ApplyAllStaged(ctx, objects, applyOpts)
	if err != nil {
		return err
	}

	// Filter out the resources that have changed.
	for _, change := range changeSet.Entries {
		if hasChanged(change.Action) {
			resultSet.Add(change)
			changeSetLog.WriteString(change.String() + "\n")
		}
	}

	// Log the changeset.
	if len(resultSet.Entries) > 0 {
		log.Info("Server-side apply completed",
			"output", resultSet.ToMap())
	}

	// Create an inventory from the reconciled resources.
	newInventory := inventory.New()
	err = inventory.AddChangeSet(newInventory, changeSet)
	if err != nil {
		return err
	}

	// Set last applied inventory in status.
	obj.Status.Inventory = newInventory

	// Detect stale resources which are subject to garbage collection.
	staleObjects, err := inventory.Diff(oldInventory, newInventory)
	if err != nil {
		return err
	}

	// Garbage collect stale resources.
	if len(staleObjects) > 0 {
		deleteOpts := ssa.DeleteOptions{
			PropagationPolicy: metav1.DeletePropagationBackground,
			Inclusions:        resourceManager.GetOwnerLabels(obj.Name, obj.Namespace),
			Exclusions: map[string]string{
				fluxcdv1.PruneAnnotation: fluxcdv1.DisabledValue,
			},
		}

		deleteSet, err := resourceManager.DeleteAll(ctx, staleObjects, deleteOpts)
		if err != nil {
			return err
		}

		if len(deleteSet.Entries) > 0 {
			for _, change := range deleteSet.Entries {
				changeSetLog.WriteString(change.String() + "\n")
			}
			log.Info("Garbage collection completed",
				"output", deleteSet.ToMap())
		}
	}

	// Emit event only if the server-side apply resulted in changes.
	applyLog := strings.TrimSuffix(changeSetLog.String(), "\n")
	if applyLog != "" {
		r.EventRecorder.Event(obj,
			corev1.EventTypeNormal,
			"ApplySucceeded",
			applyLog)
	}

	// Wait for the resources to become ready.
	if obj.Spec.Wait && len(resultSet.Entries) > 0 {
		if err := resourceManager.WaitForSet(resultSet.ToObjMetadataSet(), ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  obj.GetTimeout(),
			FailFast: true,
		}); err != nil {
			return err
		}
		log.Info("Health check completed")
	}

	return nil
}

// finalizeStatus updates the object status and conditions.
func (r *ResourceGroupReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.ResourceGroup,
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

// uninstall deletes all the resources managed by the ResourceGroup.
//
//nolint:unparam
func (r *ResourceGroupReconciler) uninstall(ctx context.Context,
	obj *fluxcdv1.ResourceGroup) (ctrl.Result, error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	if obj.IsDisabled() || obj.Status.Inventory == nil || len(obj.Status.Inventory.Entries) == 0 {
		controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
		return ctrl.Result{}, nil
	}

	resourceManager := ssa.NewResourceManager(r.Client, nil, ssa.Owner{
		Field: r.StatusManager,
		Group: fluxcdv1.GroupVersion.Group,
	})

	opts := ssa.DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
		Inclusions:        resourceManager.GetOwnerLabels(obj.Name, obj.Namespace),
		Exclusions: map[string]string{
			fluxcdv1.PruneAnnotation: fluxcdv1.DisabledValue,
		},
	}

	objects, _ := inventory.List(obj.Status.Inventory)

	changeSet, err := resourceManager.DeleteAll(ctx, objects, opts)
	if err != nil {
		log.Error(err, "pruning for deleted resource failed")
	}

	controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
	msg := fmt.Sprintf("Uninstallation completed in %v", fmtDuration(reconcileStart))
	log.Info(msg, "output", changeSet.ToMap())

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// patch updates the object status, conditions and finalizers.
func (r *ResourceGroupReconciler) patch(ctx context.Context,
	obj *fluxcdv1.ResourceGroup,
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

// requeueAfterResourceGroup returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations.
func requeueAfterResourceGroup(obj *fluxcdv1.ResourceGroup) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result
}