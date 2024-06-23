// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
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
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
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

	obj := &fluxcdv1.FluxInstance{}
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

		if err := r.recordMetrics(obj); err != nil {
			log.Error(err, "failed to record metrics")
		}

		if err := reporter.RequestReportUpdate(ctx,
			r.Client, fluxcdv1.DefaultInstanceName,
			r.StatusManager, obj.Namespace); err != nil {
			log.Error(err, "failed to request report update")
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
		msg := "Reconciliation in progress"
		conditions.MarkUnknown(obj,
			meta.ReadyCondition,
			meta.ProgressingReason,
			msg)
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			msg)
		return ctrl.Result{Requeue: true}, nil
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		msg := "Reconciliation in disabled"
		log.Error(errors.New("can't reconcile instance"), msg)
		r.Event(obj, corev1.EventTypeWarning, "ReconciliationDisabled", msg)
		return ctrl.Result{}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *FluxInstanceReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
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
	if err := r.patch(ctx, obj, patcher); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	tmpDir, err := builder.MkdirTempAbs("", "flux")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create tmp dir: %w", err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Error(err, "failed to remove tmp dir", "dir", tmpDir)
		}
	}()

	// Fetch the distribution manifests.
	manifestsDir, err := r.fetch(ctx, obj, tmpDir)
	if err != nil {
		msg := fmt.Sprintf("fetch failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ArtifactFailedReason,
			msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.ArtifactFailedReason, msg)
		return ctrl.Result{}, err
	}

	// Build the distribution manifests.
	buildResult, err := r.build(ctx, obj, manifestsDir)
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
		msg := fmt.Sprintf("Upgrading to revision %s", buildResult.Revision)
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

	// Apply the distribution manifests.
	if err := r.apply(ctx, obj, buildResult); err != nil {
		msg := fmt.Sprintf("reconciliation failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			msg)
		r.EventRecorder.Event(obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)

		return ctrl.Result{}, err
	}

	// Mark the object as ready.
	obj.Status.LastAppliedRevision = obj.Status.LastAttemptedRevision
	msg = fmt.Sprintf("Reconciliation finished in %s", fmtDuration(reconcileStart))
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		msg)
	log.Info(msg, "revision", obj.Status.LastAppliedRevision)
	r.EventRecorder.AnnotatedEventf(obj,
		map[string]string{fluxcdv1.RevisionAnnotation: obj.Status.LastAppliedRevision},
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfter(obj), nil
}

// fetch pulls the distribution OCI artifact and
// extracts the manifests to the temporary directory.
// If the distribution artifact URL is not provided,
// it falls back  to the manifests stored in the container storage.
func (r *FluxInstanceReconciler) fetch(ctx context.Context,
	obj *fluxcdv1.FluxInstance, tmpDir string) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	artifactURL := obj.Spec.Distribution.Artifact

	// Pull the latest manifests from the OCI repository.
	if artifactURL != "" {
		ctxPull, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		artifactDigest, err := builder.PullArtifact(ctxPull, artifactURL, tmpDir)
		if err != nil {
			return "", err
		}
		log.Info("fetched latest manifests", "url", artifactURL, "digest", artifactDigest)
		return tmpDir, nil
	}

	// Fall back to the manifests stored in container storage.
	return r.StoragePath, nil
}

// build reads the distribution manifests from the local storage,
// matches the version and builds the final resources.
func (r *FluxInstanceReconciler) build(ctx context.Context,
	obj *fluxcdv1.FluxInstance, manifestsDir string) (*builder.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	fluxManifestsDir := filepath.Join(manifestsDir, "flux")
	if _, err := os.Stat(fluxManifestsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("storage path %s does not exist", fluxManifestsDir)
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

	ver, err := builder.MatchVersion(fluxManifestsDir, obj.Spec.Distribution.Version)
	if err != nil {
		return nil, err
	}

	if obj.Status.LastAppliedRevision != "" {
		if err := builder.IsCompatibleVersion(obj.Status.LastAppliedRevision, ver); err != nil {
			return nil, err
		}
	}

	options := builder.MakeDefaultOptions()
	options.Version = ver
	options.Registry = obj.GetDistribution().Registry
	options.ImagePullSecret = obj.GetDistribution().ImagePullSecret
	options.Namespace = obj.GetNamespace()
	options.Components = obj.GetComponents()
	options.ClusterDomain = obj.GetCluster().Domain
	options.NetworkPolicy = obj.GetCluster().NetworkPolicy

	if obj.GetCluster().Type == "openshift" {
		options.Patches += builder.ProfileOpenShift
	}
	if obj.GetCluster().Multitenant {
		options.Patches += builder.ProfileMultitenant
	}

	if obj.Spec.Storage != nil {
		options.ArtifactStorage = &builder.ArtifactStorage{
			Class: obj.Spec.Storage.Class,
			Size:  obj.Spec.Storage.Size,
		}
	}

	if obj.Spec.Sync != nil {
		options.Sync = &builder.Sync{
			Kind:       obj.Spec.Sync.Kind,
			Interval:   obj.Spec.Sync.Interval.Duration.String(),
			Ref:        obj.Spec.Sync.Ref,
			PullSecret: obj.Spec.Sync.PullSecret,
			URL:        obj.Spec.Sync.URL,
			Path:       obj.Spec.Sync.Path,
		}
	}

	if obj.Spec.Kustomize != nil && len(obj.Spec.Kustomize.Patches) > 0 {
		patchesData, err := yaml.Marshal(obj.Spec.Kustomize.Patches)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kustomize patches: %w", err)
		}
		options.Patches += string(patchesData)
	}

	srcDir := filepath.Join(fluxManifestsDir, ver)
	images, err := builder.ExtractComponentImagesWithDigest(filepath.Join(manifestsDir, "flux-images"), options)
	if err != nil {
		log.Error(err, "falling back to extracting images from manifests")
		images, err = builder.ExtractComponentImages(srcDir, options)
		if err != nil {
			return nil, fmt.Errorf("failed to extract container images from manifests: %w", err)
		}
	}
	options.ComponentImages = images

	return builder.Build(srcDir, tmpDir, options)
}

// apply reconciles the resources in the cluster by performing
// a server-side apply, pruning of stale resources and waiting
// for the resources to become ready.
func (r *FluxInstanceReconciler) apply(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
	buildResult *builder.Result) error {
	log := ctrl.LoggerFrom(ctx)
	objects := buildResult.Objects
	var changeSetLog strings.Builder

	// Create a snapshot of the current inventory.
	oldInventory := inventory.New()
	if obj.Status.Inventory != nil {
		obj.Status.Inventory.DeepCopyInto(oldInventory)
	}

	// Create a resource manager to reconcile the resources.
	resourceManager := ssa.NewResourceManager(r.Client, r.StatusPoller, ssa.Owner{
		Field: r.StatusManager,
		Group: fluxcdv1.GroupVersion.Group,
	})
	resourceManager.SetOwnerLabels(objects, obj.GetName(), obj.GetNamespace())

	if err := normalize.UnstructuredList(objects); err != nil {
		return err
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
			{
				Name:          "flux-controller",
				OperationType: metav1.ManagedFieldsOperationApply,
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
			"output", resultSet.ToMap(), "revision", buildResult.Revision)
	}

	// Create an inventory from the reconciled resources.
	newInventory := inventory.New()
	err = inventory.AddChangeSet(newInventory, changeSet)
	if err != nil {
		return err
	}

	// Set last applied inventory in status.
	obj.Status.Inventory = newInventory
	obj.Status.Components = make([]fluxcdv1.ComponentImage, len(buildResult.ComponentImages))
	for i, img := range buildResult.ComponentImages {
		obj.Status.Components[i] = fluxcdv1.ComponentImage{
			Name:       img.Name,
			Repository: img.Repository,
			Tag:        img.Tag,
			Digest:     img.Digest,
		}
	}

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
				"output", deleteSet.ToMap(), "revision", buildResult.Revision)
		}
	}

	// Wait for the resources to become ready.
	if obj.Spec.Wait && len(resultSet.Entries) > 0 {
		if err := resourceManager.WaitForSet(resultSet.ToObjMetadataSet(), ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  obj.GetTimeout(),
		}); err != nil {
			return err
		}
		log.Info("Health check completed", "revision", buildResult.Revision)
	}

	return nil
}

// finalizeStatus updates the object status and conditions.
func (r *FluxInstanceReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
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

// patch updates the object status, conditions and finalizers.
func (r *FluxInstanceReconciler) patch(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
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

// hasChanged evaluates the given action and returns true
// if the action type matches a resource mutation or deletion.
func hasChanged(action ssa.Action) bool {
	switch action {
	case ssa.SkippedAction:
		return false
	case ssa.UnchangedAction:
		return false
	default:
		return true
	}
}

// requeueAfter returns a ctrl.Result with the requeue time set to the
// interval specified in the object's annotations.
func requeueAfter(obj *fluxcdv1.FluxInstance) ctrl.Result {
	result := ctrl.Result{}
	if obj.GetInterval() > 0 {
		result.RequeueAfter = obj.GetInterval()
	}

	return result
}

// fmtDuration returns a human-readable string
// representation of the time duration.
func fmtDuration(t time.Time) string {
	if time.Since(t) < time.Second {
		return time.Since(t).Round(time.Millisecond).String()
	} else {
		return time.Since(t).Round(time.Second).String()
	}
}

func (r *FluxInstanceReconciler) recordMetrics(obj *fluxcdv1.FluxInstance) error {
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		reporter.ResetMetrics(fluxcdv1.FluxInstanceKind)
		return nil
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	reporter.RecordMetrics(unstructured.Unstructured{Object: rawMap})
	return nil
}
