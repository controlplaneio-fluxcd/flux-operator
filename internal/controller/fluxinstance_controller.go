// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
	"github.com/fluxcd/pkg/apis/meta"
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
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// FluxInstanceReconciler reconciles a FluxInstance object
type FluxInstanceReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	Scheme        *runtime.Scheme
	ClusterReader engine.ClusterReaderFactory

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
		initializeObjectStatus(obj)
		return ctrl.Result{Requeue: true}, nil
	}

	// Pause reconciliation if the object has the reconcile annotation set to 'disabled'.
	if obj.IsDisabled() {
		log.Error(errors.New("can't reconcile instance"), fluxcdv1.ReconciliationDisabledMessage)
		r.Event(obj, corev1.EventTypeWarning, fluxcdv1.ReconciliationDisabledReason, fluxcdv1.ReconciliationDisabledMessage)
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
	conditions.MarkUnknown(obj,
		meta.ReadyCondition,
		meta.ProgressingReason,
		"%s", msgInProgress)
	conditions.MarkReconciling(obj,
		meta.ProgressingReason,
		"%s", msgInProgress)
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
	manifestsDir, artifactDigest, err := r.fetch(ctx, obj, tmpDir)
	if err != nil {
		msg := fmt.Sprintf("fetch failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ArtifactFailedReason,
			"%s", msg)
		r.notify(ctx, obj, meta.ArtifactFailedReason, corev1.EventTypeWarning, msg)
		return ctrl.Result{}, err
	}

	// Sanity check for building the distribution manifests.
	if err := builder.PreflightChecks(r.StoragePath,
		builder.WithMinVersion("2.2.0"),
		builder.WithContainerOS("distroless", 12),
		builder.WithContainerOS("rhel", 8),
	); err != nil {
		// If this happens, then the operator image has been tampered with
		// e.g.: the manifests are missing, or the Flux / OS version is not supported.
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.BuildFailedReason,
			"%s", err.Error())
		conditions.MarkStalled(obj,
			meta.BuildFailedReason,
			"%s", err.Error())
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	// Build the distribution manifests.
	buildResult, err := r.build(ctx, obj, manifestsDir)
	if err != nil {
		msg := fmt.Sprintf("build failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.BuildFailedReason,
			"%s", msg)
		conditions.MarkStalled(obj,
			meta.BuildFailedReason,
			"%s", msg)

		// Track build failure in history using the spec digest.
		specData, _ := json.Marshal(obj.Spec)
		specDigest := digest.FromString(string(specData)).String()
		obj.Status.History.Upsert(specDigest,
			time.Now(),
			time.Since(reconcileStart),
			conditions.GetReason(obj, meta.ReadyCondition),
			nil)

		r.notify(ctx, obj, meta.BuildFailedReason, corev1.EventTypeWarning, msg)
		return ctrl.Result{}, reconcile.TerminalError(err)
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

	// Extract the revision digest and version from the build result.
	version, revisionDigest, err := builder.ExtractVersionDigest(buildResult.Revision)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse revision %s: %w", buildResult.Revision, err)
	}

	// Apply the distribution manifests.
	if err := r.apply(ctx, obj, buildResult); err != nil {
		msg := fmt.Sprintf("reconciliation failed: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.ReconciliationFailedReason,
			"%s", msg)

		// Track apply failure in history using the revision digest.
		obj.Status.History.Upsert(revisionDigest,
			time.Now(),
			time.Since(reconcileStart),
			conditions.GetReason(obj, meta.ReadyCondition),
			map[string]string{
				"flux": version,
			})

		r.notify(ctx, obj, meta.ReconciliationFailedReason, corev1.EventTypeWarning, msg)
		return ctrl.Result{}, err
	}

	// Mark the object as ready.
	obj.Status.LastAppliedRevision = obj.Status.LastAttemptedRevision
	obj.Status.LastArtifactRevision = artifactDigest
	msg := reconcileMessage(reconcileStart)
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)

	// Track successful reconciliation in history.
	obj.Status.History.Upsert(revisionDigest,
		time.Now(),
		time.Since(reconcileStart),
		conditions.GetReason(obj, meta.ReadyCondition),
		map[string]string{
			"flux": version,
		})

	log.Info(msg, "revision", obj.Status.LastAppliedRevision)
	r.EventRecorder.AnnotatedEventf(obj,
		map[string]string{fluxcdv1.RevisionAnnotation: obj.Status.LastAppliedRevision},
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		"%s", msg)

	return requeueAfter(obj), nil
}

// fetch pulls the distribution OCI artifact and
// extracts the manifests to the temporary directory.
// If the distribution artifact URL is not provided,
// it falls back  to the manifests stored in the container storage.
func (r *FluxInstanceReconciler) fetch(ctx context.Context,
	obj *fluxcdv1.FluxInstance, tmpDir string) (string, string, error) {
	log := ctrl.LoggerFrom(ctx)
	artifactURL := obj.Spec.Distribution.Artifact

	// Pull the latest manifests from the OCI repository.
	if artifactURL != "" {
		ctxPull, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create a keychain for the distribution artifact.
		keyChain, err := GetDistributionKeychain(ctxPull, r.Client, obj)
		if err != nil {
			return "", "", err
		}
		artifactDigest, err := builder.PullArtifact(ctxPull, artifactURL, tmpDir, keyChain)
		if err != nil {
			return "", "", err
		}
		log.Info("fetched latest manifests", "url", artifactURL, "digest", artifactDigest)
		return tmpDir, artifactDigest, nil
	}

	// Fall back to the manifests stored in container storage.
	return r.StoragePath, "", nil
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

	latestVer, err := builder.MatchVersion(fluxManifestsDir, "2.x")
	if err != nil {
		return nil, err
	}

	if ver != latestVer {
		msg := fmt.Sprintf("Flux %s is outdated, the latest stable version is %s", ver, latestVer)
		r.EventRecorder.Event(obj, corev1.EventTypeNormal, fluxcdv1.OutdatedReason, msg)
		log.Info(msg)
	}

	options := builder.MakeDefaultOptions()
	options.Version = ver
	options.Registry = obj.GetDistribution().Registry
	options.ImagePullSecret = obj.GetDistribution().ImagePullSecret
	options.Namespace = obj.GetNamespace()
	options.Components = obj.GetComponents()
	options.NetworkPolicy = obj.GetCluster().NetworkPolicy

	if obj.GetCluster().Domain != "" {
		options.ClusterDomain = obj.GetCluster().Domain
	}

	options.Patches += builder.GetProfileClusterType(obj.GetCluster().Type)
	options.Patches += builder.GetProfileClusterSize(obj.GetCluster().Size)

	if obj.GetCluster().Multitenant {
		options.Patches += builder.GetProfileMultitenant(obj.GetCluster().TenantDefaultServiceAccount)
	}

	if err := options.ValidateAndPatchComponents(); err != nil {
		return nil, err
	}

	if err := options.ValidateAndApplyWorkloadIdentityConfig(obj.GetCluster()); err != nil {
		return nil, err
	}

	if obj.Spec.Sharding != nil {
		options.ShardingKey = obj.Spec.Sharding.Key
		options.Shards = obj.Spec.Sharding.Shards
		options.ShardingStorage = obj.IsShardingStorageEnabled()
	}

	if obj.Spec.Storage != nil {
		options.ArtifactStorage = &builder.ArtifactStorage{
			Class: obj.Spec.Storage.Class,
			Size:  obj.Spec.Storage.Size,
		}
	}

	if obj.Spec.Sync != nil {
		syncName := obj.GetNamespace()
		if obj.Spec.Sync.Name != "" {
			syncName = obj.Spec.Sync.Name
		}
		options.Sync = &builder.Sync{
			Name:       syncName,
			Kind:       obj.Spec.Sync.Kind,
			Interval:   obj.Spec.Sync.Interval.Duration.String(),
			Ref:        obj.Spec.Sync.Ref,
			PullSecret: obj.Spec.Sync.PullSecret,
			URL:        obj.Spec.Sync.URL,
			Path:       obj.Spec.Sync.Path,
			Provider:   obj.Spec.Sync.Provider,
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

	// Configure the Kubernetes client for impersonation.
	var impersonatorOpts []runtimeClient.ImpersonatorOption
	if r.ClusterReader != nil {
		impersonatorOpts = append(impersonatorOpts, runtimeClient.WithPolling(r.ClusterReader))
	}
	impersonation := runtimeClient.NewImpersonator(r.Client, impersonatorOpts...)

	// Create the Kubernetes client that runs under impersonation.
	kubeClient, statusPoller, err := impersonation.GetClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to build kube client: %w", err)
	}

	// Create a resource manager to reconcile the resources.
	resourceManager := ssa.NewResourceManager(kubeClient, statusPoller, ssa.Owner{
		Field: r.StatusManager,
		Group: fluxcdv1.GroupVersion.Group,
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
		FieldManagers: takeOwnershipFrom([]string{"flux"}),
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
	if obj.GetWait() && len(changeSet.Entries) > 0 {
		if err := resourceManager.WaitForSetWithContext(ctx, changeSet.ToObjMetadataSet(), ssa.WaitOptions{
			Interval: 5 * time.Second,
			Timeout:  obj.GetTimeout(),
		}); err != nil {
			readyStatus := aggregateNotReadyStatus(ctx, kubeClient, objects)
			return fmt.Errorf("%w\n%s", err, readyStatus)
		}
		log.Info("Health check completed", "revision", buildResult.Revision)
	}

	// Check if we need to force the migration of
	// all resources regardless of their storage version.
	force := meta.ShouldHandleForceRequest(obj)

	// Migrate all custom resources if the Flux CRDs storage version has changed.
	if obj.GetMigrateResources() {
		// Force migration if this is a minor upgrade.
		if minor, err := builder.IsMinorUpgrade(obj.Status.LastAppliedRevision, buildResult.Revision); err != nil && minor {
			force = true
		}
		if err := r.migrateResources(ctx, client.MatchingLabels{"app.kubernetes.io/part-of": obj.Name}, force); err != nil {
			log.Error(err, "failed to migrate resources to the latest storage version")
		}
	}

	// Send event to notification-controller only if the server-side apply resulted in changes.
	applyLog := strings.TrimSuffix(changeSetLog.String(), "\n")
	if applyLog != "" {
		action := "updated"
		if len(oldInventory.Entries) == 0 {
			action = "installed"
		}

		ver := strings.Split(buildResult.Revision, "@")[0]

		msg := fmt.Sprintf("Flux %s %s\n%s", ver, action, applyLog)
		r.notify(ctx, obj, meta.ReconciliationSucceededReason, corev1.EventTypeNormal, msg)
	}

	return nil
}

// finalizeStatus updates the object status and conditions.
func (r *FluxInstanceReconciler) finalizeStatus(ctx context.Context,
	obj *fluxcdv1.FluxInstance,
	patcher *patch.SerialPatcher) error {
	finalizeObjectStatus(obj)

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

func (r *FluxInstanceReconciler) notify(ctx context.Context, obj *fluxcdv1.FluxInstance, reason, eventType, msg string) {
	annotations := map[string]string{}
	if obj.Status.LastAttemptedRevision != "" {
		annotations[fluxcdv1.RevisionAnnotation] = obj.Status.LastAttemptedRevision
	}
	notifier.
		New(ctx, r.EventRecorder, r.Scheme, notifier.WithFluxInstance(obj)).
		AnnotatedEventf(obj, annotations, eventType, reason, "%s", msg)
}
