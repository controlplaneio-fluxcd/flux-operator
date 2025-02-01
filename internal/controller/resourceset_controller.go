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
	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/cel"
	runtimeClient "github.com/fluxcd/pkg/runtime/client"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

// ResourceSetReconciler reconciles a ResourceSet object
type ResourceSetReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	APIReader             client.Reader
	Scheme                *runtime.Scheme
	StatusPoller          *polling.StatusPoller
	StatusManager         string
	DefaultServiceAccount string
}

// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fluxcd.controlplane.io,resources=resourcesets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx)

	obj := &fluxcdv1.ResourceSet{}
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
		log.Error(errors.New("can't reconcile instance"), fluxcdv1.ReconciliationDisabledMessage)
		r.Event(obj, corev1.EventTypeWarning, fluxcdv1.ReconciliationDisabledReason, fluxcdv1.ReconciliationDisabledMessage)
		return ctrl.Result{}, nil
	}

	// Build dependency expressions and fail terminally if the expressions are invalid.
	exprs, err := r.buildDependencyExpressions(obj)
	if err != nil {
		const msg = "Reconciliation failed terminally due to configuration error"
		errMsg := fmt.Sprintf("%s: %v", msg, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, meta.InvalidCELExpressionReason, "%s", errMsg)
		conditions.MarkStalled(obj, meta.InvalidCELExpressionReason, "%s", errMsg)
		log.Error(err, msg)
		r.Event(obj, corev1.EventTypeWarning, meta.InvalidCELExpressionReason, errMsg)
		return ctrl.Result{}, nil
	}

	// Check dependencies and requeue the reconciliation if the check fails.
	if err := r.checkDependencies(ctx, obj, exprs); err != nil {
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

func (r *ResourceSetReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
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

	// Compute the final inputs from providers and in-line inputs.
	inputs, err := r.getInputs(ctx, obj)
	if err != nil {
		msg := fmt.Sprintf("failed to compute inputs: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			"%s", msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.BuildFailedReason, msg)
		return ctrl.Result{}, err
	}

	var objects []*unstructured.Unstructured
	if len(obj.Spec.InputsFrom) > 0 && len(inputs) == 0 {
		// If providers return no inputs, we should reconcile an empty set to trigger GC.
		log.Info("No inputs returned from providers, reconciling an empty set")
	} else {
		// Build the resources using the inputs.
		buildResult, err := builder.BuildResourceSet(obj.Spec.ResourcesTemplate, obj.Spec.Resources, inputs)
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
		objects = buildResult
	}

	// Apply the resources to the cluster.
	applySetDigest, err := r.apply(ctx, obj, objects)
	if err != nil {
		msg := fmt.Sprintf("reconciliation failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			"%s", msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)

		return ctrl.Result{}, err
	}

	// Mark the object as ready and set the last applied revision.
	obj.Status.LastAppliedRevision = applySetDigest
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

	return requeueAfterResourceSet(obj), nil
}

func (r *ResourceSetReconciler) buildDependencyExpressions(obj *fluxcdv1.ResourceSet) ([]*cel.Expression, error) {
	exprs := make([]*cel.Expression, len(obj.Spec.DependsOn))
	for i, dep := range obj.Spec.DependsOn {
		if dep.Ready && dep.ReadyExpr != "" {
			expr, err := cel.NewExpression(dep.ReadyExpr)
			if err != nil {
				depMd := object.ObjMetadata{
					Namespace: dep.Namespace,
					Name:      dep.Name,
					GroupKind: schema.GroupKind{Kind: dep.Kind},
				}
				return nil, fmt.Errorf("failed to parse expression for dependency %s/%s: %w", dep.APIVersion, ssautil.FmtObjMetadata(depMd), err)
			}
			exprs[i] = expr
		}
	}
	return exprs, nil
}

func (r *ResourceSetReconciler) checkDependencies(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
	exprs []*cel.Expression) error {

	for i, dep := range obj.Spec.DependsOn {
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
		depMd := object.ObjMetadata{
			Namespace: dep.Namespace,
			Name:      dep.Name,
			GroupKind: schema.GroupKind{Kind: dep.Kind},
		}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(depObj), depObj); err != nil {
			return fmt.Errorf("dependency %s/%s not found: %w", dep.APIVersion, ssautil.FmtObjMetadata(depMd), err)
		}

		if dep.Ready {
			if dep.ReadyExpr != "" {
				isReady, err := exprs[i].EvaluateBoolean(ctx, depObj.UnstructuredContent())
				if err != nil {
					return err
				}

				if !isReady {
					return fmt.Errorf("dependency %s/%s not ready: expression '%s'", dep.APIVersion, ssautil.FmtObjMetadata(depMd), dep.ReadyExpr)
				}
			} else {
				stat, err := status.Compute(depObj)
				if err != nil {
					return fmt.Errorf("dependency %s/%s not ready: %w", dep.APIVersion, ssautil.FmtObjMetadata(depMd), err)
				}

				if stat.Status != status.CurrentStatus {
					return fmt.Errorf("dependency %s/%s not ready: status %s", dep.APIVersion, ssautil.FmtObjMetadata(depMd), stat.Status)
				}
			}
		}
	}

	return nil
}

func (r *ResourceSetReconciler) getInputs(ctx context.Context,
	obj *fluxcdv1.ResourceSet) ([]map[string]any, error) {
	providers := make([]fluxcdv1.InputProvider, 0)
	providers = append(providers, obj)
	for _, inputSource := range obj.Spec.InputsFrom {
		var provider fluxcdv1.InputProvider
		key := client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      inputSource.Name,
		}

		switch inputSource.Kind {
		case fluxcdv1.ResourceSetInputProviderKind:
			var rsip fluxcdv1.ResourceSetInputProvider
			if err := r.Get(ctx, key, &rsip); err != nil {
				return nil, fmt.Errorf("failed to get provider %s/%s: %w", key.Namespace, key.Name, err)
			}
			provider = &rsip
		default:
			return nil, fmt.Errorf("unsupported provider kind %s", inputSource.Kind)
		}

		providers = append(providers, provider)
	}

	inputs := make([]map[string]any, 0)
	for _, provider := range providers {
		exportedInputs, err := provider.GetInputs()
		if err != nil {
			return nil, fmt.Errorf("failed to get inputs from %s/%s: %w",
				provider.GroupVersionKind().Kind, provider.GetName(), err)
		}
		inputs = append(inputs, exportedInputs...)
	}

	return inputs, nil
}

// apply reconciles the resources in the cluster by performing
// a server-side apply, pruning of stale resources and waiting
// for the resources to become ready.
// It returns an error if the apply operation fails, otherwise
// it returns the sha256 digest of the applied resources.
func (r *ResourceSetReconciler) apply(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
	objects []*unstructured.Unstructured) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	var changeSetLog strings.Builder

	// Create a snapshot of the current inventory.
	oldInventory := inventory.New()
	if obj.Status.Inventory != nil {
		obj.Status.Inventory.DeepCopyInto(oldInventory)
	}

	// Configure the Kubernetes client for impersonation.
	impersonation := runtimeClient.NewImpersonator(
		r.Client,
		r.StatusPoller,
		polling.Options{},
		nil,
		runtimeClient.KubeConfigOptions{},
		r.DefaultServiceAccount,
		obj.Spec.ServiceAccountName,
		obj.GetNamespace(),
	)

	// Create the Kubernetes client that runs under impersonation.
	kubeClient, statusPoller, err := impersonation.GetClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build kube client: %w", err)
	}

	// Create a resource manager to reconcile the resources.
	resourceManager := ssa.NewResourceManager(kubeClient, statusPoller, ssa.Owner{
		Field: r.StatusManager,
		Group: fmt.Sprintf("resourceset.%s", fluxcdv1.GroupVersion.Group),
	})
	resourceManager.SetOwnerLabels(objects, obj.GetName(), obj.GetNamespace())

	if err := normalize.UnstructuredList(objects); err != nil {
		return "", err
	}

	if cm := obj.Spec.CommonMetadata; cm != nil {
		ssautil.SetCommonMetadata(objects, cm.Labels, cm.Annotations)
	}

	// Compute the sha256 digest of the resources.
	data, err := ssautil.ObjectsToYAML(objects)
	if err != nil {
		return "", fmt.Errorf("failed to convert objects to YAML: %w", err)
	}
	applySetDigest := digest.FromString(data).String()

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
		return "", err
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
		return "", err
	}

	// Set last applied inventory in status.
	obj.Status.Inventory = newInventory

	// Detect stale resources which are subject to garbage collection.
	staleObjects, err := inventory.Diff(oldInventory, newInventory)
	if err != nil {
		return "", err
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

		deleteSet, err := r.deleteAllStaged(ctx, resourceManager, staleObjects, deleteOpts)
		if err != nil {
			return "", err
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
	if obj.Spec.Wait && len(changeSet.Entries) > 0 {
		if err := resourceManager.WaitForSet(changeSet.ToObjMetadataSet(), ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  obj.GetTimeout(),
			FailFast: true,
		}); err != nil {
			readyStatus := r.aggregateNotReadyStatus(ctx, kubeClient, objects)
			return "", fmt.Errorf("%w\n%s", err, readyStatus)
		}
		log.Info("Health check completed")
	}

	return applySetDigest, nil
}

// aggregateNotReadyStatus returns the status of the Flux resources not ready.
func (r *ResourceSetReconciler) aggregateNotReadyStatus(ctx context.Context,
	kubeClient client.Client, objects []*unstructured.Unstructured) string {
	var result strings.Builder
	for _, res := range objects {
		if strings.HasSuffix(res.GetObjectKind().GroupVersionKind().Group, ".fluxcd.io") {
			if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(res), res); err == nil {
				if obj, err := status.GetObjectWithConditions(res.Object); err == nil {
					for _, cond := range obj.Status.Conditions {
						if cond.Type == meta.ReadyCondition && cond.Status != corev1.ConditionTrue {
							result.WriteString(fmt.Sprintf("%s status: %s\n", ssautil.FmtUnstructured(res), cond.Message))
						}
					}
				}
			}
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// deleteAllStaged removes resources in stages, first the Flux resources and then the rest.
// This is to ensure that the Flux GC can run under impersonation as the service account
// and role bindings are deleted after the Flux resources.
func (r *ResourceSetReconciler) deleteAllStaged(ctx context.Context,
	rm *ssa.ResourceManager,
	objects []*unstructured.Unstructured,
	opts ssa.DeleteOptions) (*ssa.ChangeSet, error) {
	log := ctrl.LoggerFrom(ctx)
	changeSet := ssa.NewChangeSet()

	var fluxObjects []*unstructured.Unstructured
	var nativeObjects []*unstructured.Unstructured
	for _, res := range objects {
		if strings.HasPrefix(res.GetAPIVersion(), "kustomize.toolkit.fluxcd.io/") ||
			strings.HasPrefix(res.GetAPIVersion(), "helm.toolkit.fluxcd.io/") {
			fluxObjects = append(fluxObjects, res)
		} else {
			nativeObjects = append(nativeObjects, res)
		}
	}

	// Delete the Flux resources first and wait for them to be removed.
	if len(fluxObjects) > 0 {
		fluxChangeSet, err := rm.DeleteAll(ctx, fluxObjects, opts)
		if err != nil {
			return changeSet, err
		}
		changeSet.Append(fluxChangeSet.Entries)

		waitErr := rm.WaitForTermination(fluxObjects, ssa.DefaultWaitOptions())
		if waitErr != nil {
			log.Error(waitErr, "failed to wait for Flux resources to be deleted")
		}
	}

	// Delete the rest of the resources.
	if len(nativeObjects) > 0 {
		nativeChangeSet, err := rm.DeleteAll(ctx, nativeObjects, opts)
		if err != nil {
			return changeSet, err
		}
		changeSet.Append(nativeChangeSet.Entries)
	}

	return changeSet, nil
}

// finalizeStatus updates the object status and conditions.
func (r *ResourceSetReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
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

// uninstall deletes all the resources managed by the ResourceSet.
//
//nolint:unparam
func (r *ResourceSetReconciler) uninstall(ctx context.Context,
	obj *fluxcdv1.ResourceSet) (ctrl.Result, error) {
	reconcileStart := time.Now()
	log := ctrl.LoggerFrom(ctx)

	if obj.IsDisabled() || obj.Status.Inventory == nil || len(obj.Status.Inventory.Entries) == 0 {
		controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)
		return ctrl.Result{}, nil
	}

	// Configure the Kubernetes client for impersonation.
	impersonation := runtimeClient.NewImpersonator(
		r.Client,
		r.StatusPoller,
		polling.Options{},
		nil,
		runtimeClient.KubeConfigOptions{},
		r.DefaultServiceAccount,
		obj.Spec.ServiceAccountName,
		obj.GetNamespace(),
	)

	// Prune the managed resources if the service account is found.
	if impersonation.CanImpersonate(ctx) {
		kubeClient, _, err := impersonation.GetClient(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}

		resourceManager := ssa.NewResourceManager(kubeClient, nil, ssa.Owner{
			Field: r.StatusManager,
			Group: fmt.Sprintf("resourceset.%s", fluxcdv1.GroupVersion.Group),
		})

		opts := ssa.DeleteOptions{
			PropagationPolicy: metav1.DeletePropagationBackground,
			Inclusions:        resourceManager.GetOwnerLabels(obj.Name, obj.Namespace),
			Exclusions: map[string]string{
				fluxcdv1.PruneAnnotation: fluxcdv1.DisabledValue,
			},
		}

		objects, _ := inventory.List(obj.Status.Inventory)

		changeSet, err := r.deleteAllStaged(ctx, resourceManager, objects, opts)
		if err != nil {
			log.Error(err, "pruning for deleted resource failed")
		}

		msg := fmt.Sprintf("Uninstallation completed in %v", fmtDuration(reconcileStart))
		log.Info(msg, "output", changeSet.ToMap())
	} else {
		log.Error(errors.New("service account not found"), "skip pruning for deleted resource")
	}

	// Release the object to be garbage collected.
	controllerutil.RemoveFinalizer(obj, fluxcdv1.Finalizer)

	// Stop reconciliation as the object is being deleted.
	return ctrl.Result{}, nil
}

// patch updates the object status, conditions and finalizers.
func (r *ResourceSetReconciler) patch(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
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

// requeueAfterResourceSet returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations.
func requeueAfterResourceSet(obj *fluxcdv1.ResourceSet) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result
}
