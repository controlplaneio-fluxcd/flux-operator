// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/engine"
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
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kuberecorder "k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/builder"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/notifier"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// ResourceSetReconciler reconciles a ResourceSet object
type ResourceSetReconciler struct {
	client.Client
	kuberecorder.EventRecorder

	APIReader     client.Reader
	Scheme        *runtime.Scheme
	ClusterReader engine.ClusterReaderFactory

	StatusManager         string
	DefaultServiceAccount string

	RequeueDependency time.Duration
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

		if err := r.recordMetrics(obj); err != nil {
			log.Error(err, "failed to record metrics")
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

	// Build dependency expressions and fail terminally if the expressions are invalid.
	exprs, err := r.buildDependencyExpressions(obj)
	if err != nil {
		errMsg := fmt.Sprintf("%s: %v", msgTerminalError, err)
		conditions.MarkFalse(obj, meta.ReadyCondition, meta.InvalidCELExpressionReason, "%s", errMsg)
		conditions.MarkStalled(obj, meta.InvalidCELExpressionReason, "%s", errMsg)
		r.notify(ctx, obj, corev1.EventTypeWarning, meta.InvalidCELExpressionReason, errMsg)
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	// Check dependencies and requeue the reconciliation if the check fails.
	if err := r.checkDependencies(ctx, obj, exprs); err != nil {
		msg := fmt.Sprintf("Retrying dependency check: %s", err.Error())
		if conditions.GetReason(obj, meta.ReadyCondition) != meta.DependencyNotReadyReason {
			log.Error(err, "dependency check failed")
			r.notify(ctx, obj, corev1.EventTypeNormal, meta.DependencyNotReadyReason, msg)
		}
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.DependencyNotReadyReason,
			"%s", msg)
		return ctrl.Result{RequeueAfter: r.RequeueDependency}, nil
	}

	// Reconcile the object.
	return r.reconcile(ctx, obj, patcher)
}

func (r *ResourceSetReconciler) reconcile(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
	patcher *patch.SerialPatcher) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	// Mark the object as reconciling and remove any previous error conditions.
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

	// Compute the final inputs from providers and in-line inputs.
	inputSet, err := r.getInputs(ctx, obj)
	if err != nil {
		// Mark the object as not ready and stalled due to input failure.
		msg := fmt.Sprintf("failed to compute inputs: %s", err.Error())
		conditions.MarkFalse(obj,
			meta.ReadyCondition,
			meta.BuildFailedReason,
			"%s", msg)
		conditions.MarkStalled(obj,
			meta.BuildFailedReason,
			"%s", msg)

		// Track input failure in history using the spec digest.
		specData, _ := json.Marshal(obj.Spec)
		specDigest := digest.FromString(string(specData)).String()
		obj.Status.History.Upsert(specDigest,
			time.Now(),
			time.Since(reconcileStart),
			conditions.GetReason(obj, meta.ReadyCondition),
			nil)

		// Emit warning event and return a terminal error.
		r.notify(ctx, obj, corev1.EventTypeWarning, meta.BuildFailedReason, msg)
		return ctrl.Result{}, reconcile.TerminalError(err)
	}

	var objects []*unstructured.Unstructured
	if len(obj.Spec.InputsFrom) > 0 && len(inputSet) == 0 {
		// If providers return no inputs, we should reconcile an empty set to trigger GC.
		log.Info("No inputs returned from providers, reconciling an empty set")
	} else {
		// Build the resources using the inputs.
		buildResult, err := builder.BuildResourceSet(obj.Spec.ResourcesTemplate, obj.Spec.Resources, inputSet)
		if err != nil {
			// Mark the object as not ready and stalled due to build failure.
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

			// Emit warning event and return a terminal error.
			r.notify(ctx, obj, corev1.EventTypeWarning, meta.BuildFailedReason, msg)
			return ctrl.Result{}, reconcile.TerminalError(err)
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

		// Track apply failure in history using digest from objects.
		data, _ := ssautil.ObjectsToYAML(objects)
		applyFailureDigest := digest.FromString(data).String()
		obj.Status.History.Upsert(applyFailureDigest,
			time.Now(),
			time.Since(reconcileStart),
			conditions.GetReason(obj, meta.ReadyCondition),
			map[string]string{
				"inputs":    fmt.Sprintf("%d", len(inputSet)),
				"resources": fmt.Sprintf("%d", len(objects)),
			})

		// Emit warning event and return an error to retry with backoff the reconciliation.
		r.notify(ctx, obj, corev1.EventTypeWarning, meta.ReconciliationFailedReason, msg)
		return ctrl.Result{}, err
	}

	// Mark the object as ready and set the last applied revision.
	obj.Status.LastAppliedRevision = applySetDigest
	msg := reconcileMessage(reconcileStart)
	conditions.MarkTrue(obj,
		meta.ReadyCondition,
		meta.ReconciliationSucceededReason,
		"%s", msg)

	// Track successful reconciliation in history.
	obj.Status.History.Upsert(applySetDigest,
		time.Now(),
		time.Since(reconcileStart),
		conditions.GetReason(obj, meta.ReadyCondition),
		map[string]string{
			"inputs":    fmt.Sprintf("%d", len(inputSet)),
			"resources": fmt.Sprintf("%d", len(objects)),
		})

	// Log and emit the reconciliation success event.
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfter(obj), nil
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
			Object: map[string]any{
				"apiVersion": dep.APIVersion,
				"kind":       dep.Kind,
				"metadata": map[string]any{
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

// getInputs fetches all the referenced provider objects and appends
// the inputs from all of them. It returns the combined list of input
// maps, including the static inputs specified on the ResourceSet.
// Objects are deduplicated by their GVK, name and namespace.
// The API version, kind, name and namespace of the provider are added
// to each input map.
func (r *ResourceSetReconciler) getInputs(ctx context.Context,
	obj *fluxcdv1.ResourceSet) (inputs.Combined, error) {

	// List providers from spec.inputsFrom.
	providerMap := make(map[inputs.ProviderKey]fluxcdv1.InputProvider)
	rsipGVK := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)
	for _, inputSource := range obj.Spec.InputsFrom {

		switch {
		// Get provider by name.
		case inputSource.Name != "":

			mapKey := inputs.ProviderKey{
				GVK:       rsipGVK,
				Name:      inputSource.Name,
				Namespace: obj.GetNamespace(),
			}
			if _, found := providerMap[mapKey]; found {
				continue
			}

			objKey := client.ObjectKey{
				Name:      inputSource.Name,
				Namespace: obj.GetNamespace(),
			}

			var rsip fluxcdv1.ResourceSetInputProvider
			if err := r.Get(ctx, objKey, &rsip); err != nil {
				return nil, fmt.Errorf("failed to get provider %s/%s: %w", objKey.Namespace, objKey.Name, err)
			}

			rsip.SetGroupVersionKind(rsipGVK)
			providerMap[mapKey] = &rsip

		// List providers by selector.
		case inputSource.Selector != nil:

			selector, err := metav1.LabelSelectorAsSelector(inputSource.Selector)
			if err != nil {
				return nil, fmt.Errorf("failed to parse selector %s: %w", inputSource.Selector, err)
			}

			var rsipList fluxcdv1.ResourceSetInputProviderList

			listOpts := []client.ListOption{
				client.InNamespace(obj.GetNamespace()),
				client.MatchingLabelsSelector{Selector: selector},
			}

			if err := r.List(ctx, &rsipList, listOpts...); err != nil {
				return nil, fmt.Errorf("failed to list providers with selector %s: %w", inputSource.Selector, err)
			}

			for _, rsip := range rsipList.Items {
				rsip.SetGroupVersionKind(rsipGVK)
				providerMap[inputs.NewProviderKey(&rsip)] = &rsip
			}

		default:
			return nil, errors.New("input provider reference must have either name or selector set")
		}
	}

	// Ensure correct GVK for the ResourceSet object.
	obj.SetGroupVersionKind(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind))

	return inputs.Combine(obj, providerMap)
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
	var impersonatorOpts []runtimeClient.ImpersonatorOption
	if r.DefaultServiceAccount != "" || obj.Spec.ServiceAccountName != "" {
		impersonatorOpts = append(impersonatorOpts,
			runtimeClient.WithServiceAccount(r.DefaultServiceAccount, obj.Spec.ServiceAccountName, obj.GetNamespace()))
	}
	if r.ClusterReader != nil {
		impersonatorOpts = append(impersonatorOpts, runtimeClient.WithPolling(r.ClusterReader))
	}
	impersonation := runtimeClient.NewImpersonator(r.Client, impersonatorOpts...)

	// Create the Kubernetes client that runs under impersonation.
	kubeClient, statusPoller, err := impersonation.GetClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build kube client: %w", err)
	}

	// Create a resource manager to reconcile the resources.
	resourceManager := ssa.NewResourceManager(kubeClient, statusPoller, ssa.Owner{
		Field: r.StatusManager,
		Group: fluxcdv1.GroupOwnerLabelResourceSet,
	})
	resourceManager.SetOwnerLabels(objects, obj.GetName(), obj.GetNamespace())

	if err := normalize.UnstructuredList(objects); err != nil {
		return "", err
	}

	if cm := obj.Spec.CommonMetadata; cm != nil {
		ssautil.SetCommonMetadata(objects, cm.Labels, cm.Annotations)
	}

	if err := r.copyResources(ctx, kubeClient, objects); err != nil {
		return "", err
	}

	// Compute the sha256 digest of the resources.
	data, err := ssautil.ObjectsToYAML(objects)
	if err != nil {
		return "", fmt.Errorf("failed to convert objects to YAML: %w", err)
	}
	applySetDigest := digest.FromString(data).String()

	applyOpts := ssa.DefaultApplyOptions()
	applyOpts.Force = obj.IsForceEnabled()
	applyOpts.ForceSelector = map[string]string{
		fluxcdv1.ForceAnnotation: fluxcdv1.EnabledValue,
	}
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
		// Take ownership of existing resources and
		// undo changes made with kubectl or helm.
		FieldManagers: takeOwnershipFrom(nil),
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
		r.notify(ctx, obj,
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
			readyStatus := aggregateNotReadyStatus(ctx, kubeClient, objects)
			return "", fmt.Errorf("%w\n%s", err, readyStatus)
		}
		log.Info("Health check completed")
	}

	return applySetDigest, nil
}

// copyResources copies data from ConfigMaps and Secrets based on the
// annotations set on the resources template.
func (r *ResourceSetReconciler) copyResources(ctx context.Context,
	kubeClient client.Client, objects []*unstructured.Unstructured) error {
	for i := range objects {
		if objects[i].GetAPIVersion() == "v1" {
			source, found := objects[i].GetAnnotations()[fluxcdv1.CopyFromAnnotation]
			if !found {
				continue
			}

			sourceParts := strings.Split(source, "/")
			if len(sourceParts) != 2 {
				return fmt.Errorf("invalid %s annotation value '%s' must be in the format 'namespace/name'",
					fluxcdv1.CopyFromAnnotation, source)
			}

			sourceName := types.NamespacedName{
				Namespace: sourceParts[0],
				Name:      sourceParts[1],
			}

			switch objects[i].GetKind() {
			case "ConfigMap":
				cm := &corev1.ConfigMap{}
				if err := kubeClient.Get(ctx, sourceName, cm); err != nil {
					return fmt.Errorf("failed to copy data from ConfigMap/%s: %w", source, err)
				}
				if err := unstructured.SetNestedStringMap(objects[i].Object, cm.Data, "data"); err != nil {
					return fmt.Errorf("failed to copy data from ConfigMap/%s: %w", source, err)
				}
			case "Secret":
				secret := &corev1.Secret{}
				if err := kubeClient.Get(ctx, sourceName, secret); err != nil {
					return fmt.Errorf("failed to copy data from Secret/%s: %w", source, err)
				}
				_, ok, err := unstructured.NestedString(objects[i].Object, "type")
				if err != nil {
					return fmt.Errorf("type field of Secret/%s is not a string: %w", source, err)
				}
				if !ok {
					if secret.Type == "" {
						secret.Type = corev1.SecretTypeOpaque
					}
					if err := unstructured.SetNestedField(objects[i].Object, string(secret.Type), "type"); err != nil {
						return fmt.Errorf("failed to copy type from Secret/%s: %w", source, err)
					}
				}
				data := make(map[string]string, len(secret.Data))
				for k, v := range secret.Data {
					data[k] = string(v)
				}
				if err := unstructured.SetNestedStringMap(objects[i].Object, data, "stringData"); err != nil {
					return fmt.Errorf("failed to copy data from Secret/%s: %w", source, err)
				}
			}
		}
	}
	return nil
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
		if strings.HasPrefix(res.GetAPIVersion(), fluxcdv1.FluxKustomizeGroup) ||
			strings.HasPrefix(res.GetAPIVersion(), fluxcdv1.FluxHelmGroup) {
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
	finalizeObjectStatus(obj)

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
	var impersonatorOpts []runtimeClient.ImpersonatorOption
	if r.DefaultServiceAccount != "" || obj.Spec.ServiceAccountName != "" {
		impersonatorOpts = append(impersonatorOpts,
			runtimeClient.WithServiceAccount(r.DefaultServiceAccount, obj.Spec.ServiceAccountName, obj.GetNamespace()))
	}
	if r.ClusterReader != nil {
		impersonatorOpts = append(impersonatorOpts, runtimeClient.WithPolling(r.ClusterReader))
	}
	impersonation := runtimeClient.NewImpersonator(r.Client, impersonatorOpts...)

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

		msg := uninstallMessage(reconcileStart)
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

func (r *ResourceSetReconciler) recordMetrics(obj *fluxcdv1.ResourceSet) error {
	if !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		reporter.DeleteMetricsFor(fluxcdv1.ResourceSetKind, obj.GetName(), obj.GetNamespace())
		return nil
	}
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	reporter.RecordMetrics(unstructured.Unstructured{Object: rawMap})
	return nil
}

func (r *ResourceSetReconciler) notify(ctx context.Context, obj *fluxcdv1.ResourceSet, eventType, reason, message string) {
	notifier.
		New(ctx, r.EventRecorder, r.Scheme, notifier.WithClient(r.Client)).
		Event(obj, eventType, reason, message)
}
