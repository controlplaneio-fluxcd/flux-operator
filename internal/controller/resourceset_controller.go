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
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/cel"
	runtimeClient "github.com/fluxcd/pkg/runtime/client"
	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/fluxcd/pkg/runtime/controller"
	"github.com/fluxcd/pkg/runtime/patch"
	"github.com/fluxcd/pkg/ssa"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	OverrideFieldManagers []string

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

	// Reject an invalid spec before the empty-inputs branch can trigger
	// garbage collection. The rules are enforced here because they cannot
	// be CRD CEL rules, see the notes on api/v1 ResourceSetSpec.
	if err := builder.ValidateResourceSetSpec(obj.Spec); err != nil {
		return r.stallWithTerminalError(ctx, obj, reconcileStart, "build failed", err)
	}

	// Compute the final inputs from providers and in-line inputs.
	inputSet, err := r.getInputs(ctx, obj)
	if err != nil {
		return r.stallWithTerminalError(ctx, obj, reconcileStart, "failed to compute inputs", err)
	}

	var steps []builder.StepBuildResult
	var objects []*unstructured.Unstructured
	if len(obj.Spec.InputsFrom) > 0 && len(inputSet) == 0 {
		// If providers return no inputs, we should reconcile an empty set to trigger GC.
		log.Info("No inputs returned from providers, reconciling an empty set")
	} else {
		// Build the resources of each step using the inputs. A steps-less
		// ResourceSet is reconciled as a single anonymous step.
		var buildErr error
		steps, buildErr = builder.BuildResourceSetFromSpec(obj.Spec, inputSet)
		if buildErr != nil {
			return r.stallWithTerminalError(ctx, obj, reconcileStart, "build failed", buildErr)
		}

		// Flatten the step objects sharing pointers with the step slices
		// for tracking the applied resources digest in history.
		objects = builder.FlattenSteps(steps)
	}

	// Compute the history metadata of the reconciliation.
	historyMetadata := map[string]string{
		"inputs":    fmt.Sprintf("%d", len(inputSet)),
		"resources": fmt.Sprintf("%d", len(objects)),
	}
	if obj.HasSteps() {
		historyMetadata["steps"] = fmt.Sprintf("%d", len(obj.Spec.Steps))
	}

	// Apply the resources to the cluster.
	applySetDigest, err := r.apply(ctx, obj, patcher, steps)
	if err != nil {
		if qesErr := new(controller.QueueEventSource); errors.As(err, &qesErr) {
			return returnHealthChecksCanceled(ctx, obj, qesErr, r.EventRecorder)
		}

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
			historyMetadata)

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
		historyMetadata)

	// Log and emit the reconciliation success event.
	log.Info(msg)
	r.EventRecorder.Event(obj,
		corev1.EventTypeNormal,
		meta.ReconciliationSucceededReason,
		msg)

	return requeueAfter(obj), nil
}

// stallWithTerminalError marks the object as not ready and stalled with the
// build failed reason and a message composed of the given prefix and error.
// It tracks the failure in history using the spec digest, emits a warning
// event and returns a terminal error to stop the retries until a spec change.
func (r *ResourceSetReconciler) stallWithTerminalError(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
	reconcileStart time.Time,
	msgPrefix string,
	err error) (ctrl.Result, error) {
	// Mark the object as not ready and stalled due to the failure.
	msg := fmt.Sprintf("%s: %s", msgPrefix, err.Error())
	conditions.MarkFalse(obj,
		meta.ReadyCondition,
		meta.BuildFailedReason,
		"%s", msg)
	conditions.MarkStalled(obj,
		meta.BuildFailedReason,
		"%s", msg)

	// Track the failure in history using the spec digest.
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
			switch {
			// Custom CEL ready expression.
			case dep.ReadyExpr != "":
				isReady, err := exprs[i].EvaluateBoolean(ctx, depObj.UnstructuredContent())
				if err != nil {
					return err
				}

				if !isReady {
					return fmt.Errorf("dependency %s/%s not ready: expression '%s'", dep.APIVersion, ssautil.FmtObjMetadata(depMd), dep.ReadyExpr)
				}
			// Built-in CEL ready expression for ResourceSet and ResourceSetInputProvider.
			case dep.Kind == fluxcdv1.ResourceSetKind || dep.Kind == fluxcdv1.ResourceSetInputProviderKind:
				expr, err := cel.NewExpression(fluxcdv1.HealthCheckExpr)
				if err != nil {
					return err
				}

				isReady, err := expr.EvaluateBoolean(ctx, depObj.UnstructuredContent())
				if err != nil {
					return err
				}

				if !isReady {
					return fmt.Errorf("dependency %s/%s not ready", dep.APIVersion, ssautil.FmtObjMetadata(depMd))
				}
			// Default status check using kstatus.
			default:
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
// a server-side apply of each step in order, waiting for the
// non-final step resources to become ready before starting the
// next step, pruning of stale resources after all steps have
// been applied, and waiting for the final step resources to
// become ready when the wait option is enabled.
// It returns an error if the apply operation fails, otherwise
// it returns the sha256 digest of the applied resources.
func (r *ResourceSetReconciler) apply(ctx context.Context,
	obj *fluxcdv1.ResourceSet,
	patcher *patch.SerialPatcher,
	steps []builder.StepBuildResult) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	var changeSetLog strings.Builder

	// Guard against an empty step list to always reconcile at least
	// the anonymous empty step which triggers garbage collection.
	if len(steps) == 0 {
		steps = []builder.StepBuildResult{{}}
	}

	// Flatten the steps into a single list of objects sharing pointers
	// with the step slices, so the in-place mutations performed below
	// are visible to the per-step apply.
	objects := builder.FlattenSteps(steps)

	// Create a snapshot of the current inventory.
	oldInventory := inventory.New()
	if obj.Status.Inventory != nil {
		obj.Status.Inventory.DeepCopyInto(oldInventory)
	}

	// Configure the Kubernetes client for impersonation.
	impersonation, err := r.makeImpersonator(obj)
	if err != nil {
		return "", err
	}

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

	if cm := obj.Spec.CommonMetadata; cm != nil {
		ssautil.SetCommonMetadata(objects, cm.Labels, cm.Annotations)
	}

	if err := r.copyResources(ctx, kubeClient, objects); err != nil {
		return "", err
	}

	if err := r.convertKubeConfigResources(ctx, kubeClient, objects); err != nil {
		return "", err
	}

	externalRefs, err := r.computeChecksumsFromAnnotations(ctx, kubeClient, objects)
	if err != nil {
		return "", err
	}
	obj.Status.ExternalChecksumRefs = externalRefs

	if err := normalize.UnstructuredList(objects); err != nil {
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
		FieldManagers: takeOwnershipFrom(r.OverrideFieldManagers),
	}
	applyOpts.CustomStageKinds = map[schema.GroupKind]struct{}{
		{Group: "rbac.authorization.k8s.io", Kind: "Role"}: {},
	}

	// Apply the resources of each step to the cluster and wait for the
	// non-final step resources to become ready before starting the next step.
	newInventory := inventory.New()
	var finalChangeSet *ssa.ChangeSet
	for i, step := range steps {
		isFinalStep := i == len(steps)-1

		// Delete the failed Jobs annotated with recreateOnFailure so
		// that the server-side apply can recreate them from scratch.
		if err := r.deleteFailedJobs(ctx, kubeClient, resourceManager, obj, step); err != nil {
			return "", stepError(step, "job recreation failed", err)
		}

		// Apply the step resources to the cluster.
		changeSet, err := resourceManager.ApplyAllStaged(ctx, step.Objects, applyOpts)
		if err != nil {
			// ApplyAllStaged returns the changeset of the already-completed
			// stages together with the error; the partial apply results must
			// enter the inventory so a failed step never orphans applied objects.
			return "", stepError(step, "apply failed",
				trackPartialApply(obj, oldInventory, newInventory, changeSet, err))
		}

		// Filter out the resources that have changed.
		resultSet := ssa.NewChangeSet()
		var stepLog strings.Builder
		for _, change := range changeSet.Entries {
			if hasChanged(change.Action) {
				resultSet.Add(change)
				stepLog.WriteString(change.String() + "\n")
			}
		}

		// Log the changeset.
		if len(resultSet.Entries) > 0 {
			log.Info("Server-side apply completed",
				"output", resultSet.ToMap())
		}

		// Track the applied resources in the inventory and keep the union
		// with the old inventory so a failure mid-sequence never orphans
		// applied objects and never loses entries owned by later steps.
		if err := inventory.AddChangeSet(newInventory, changeSet); err != nil {
			return "", err
		}
		obj.Status.Inventory = inventory.Merge(oldInventory, newInventory)

		if isFinalStep {
			// The final step event and health check run after garbage
			// collection to preserve the apply -> GC -> event -> wait order.
			finalChangeSet = changeSet
			changeSetLog.WriteString(stepLog.String())
			break
		}

		// Patch the status with the step progress and the inventory
		// before the long-running health check.
		conditions.MarkReconciling(obj,
			meta.ProgressingReason,
			"Applying step %d/%d %q", i+1, len(steps), step.Name)
		if err := r.patch(ctx, obj, patcher); err != nil {
			return "", err
		}

		// Emit the step apply event only if the server-side apply resulted in changes.
		if stepLog.Len() > 0 {
			r.notify(ctx, obj,
				corev1.EventTypeNormal,
				"ApplySucceeded",
				fmt.Sprintf("step %q: %s", step.Name, strings.TrimSuffix(stepLog.String(), "\n")))
		}

		// Wait for the step resources to become ready
		// before starting the next step.
		if len(changeSet.Entries) > 0 {
			if err := r.waitForStep(ctx, kubeClient, resourceManager, obj, step, changeSet); err != nil {
				return "", err
			}
			log.Info("Health check completed", "step", step.Name)
		}
	}

	// Detect stale resources which are subject to garbage collection.
	staleObjects, err := inventory.Diff(oldInventory, newInventory)
	if err != nil {
		return "", err
	}

	// Garbage collect stale resources after all the steps have been
	// applied and all the inter-step health checks have passed.
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
			// Keep the old and new inventory union in status so the
			// undeleted stale objects stay tracked and their deletion
			// is retried on the next reconciliation.
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

	// Drop the stale entries from the inventory only after
	// the garbage collection has fully succeeded.
	obj.Status.Inventory = newInventory

	// Emit event only if the server-side apply resulted in changes.
	applyLog := strings.TrimSuffix(changeSetLog.String(), "\n")
	if applyLog != "" {
		r.notify(ctx, obj,
			corev1.EventTypeNormal,
			"ApplySucceeded",
			applyLog)
	}

	// Wait for the final step resources to become ready. The loop above
	// always runs at least one iteration and its final iteration either
	// returns an error or sets the final changeset, so it is never nil here.
	if obj.Spec.Wait && len(finalChangeSet.Entries) > 0 {
		if err := r.waitForStep(ctx, kubeClient, resourceManager, obj, steps[len(steps)-1], finalChangeSet); err != nil {
			return "", err
		}
		log.Info("Health check completed")
	}

	return applySetDigest, nil
}

// waitForStep waits for the changeset resources of the given step to become
// ready within the step timeout, using the interrupt context derived from
// the given context to cancel the wait when a new event enqueues the object.
// On failure, it returns the enqueued object error as-is, or the health check
// error together with the aggregated not-ready status of the step's Flux resources.
func (r *ResourceSetReconciler) waitForStep(ctx context.Context,
	kubeClient client.Client,
	rm *ssa.ResourceManager,
	obj *fluxcdv1.ResourceSet,
	step builder.StepBuildResult,
	changeSet *ssa.ChangeSet) error {
	healthCtx := controller.GetInterruptContext(ctx)
	if err := rm.WaitForSetWithContext(healthCtx, changeSet.ToObjMetadataSet(), ssa.WaitOptions{
		Interval: 5 * time.Second,
		Timeout:  stepTimeout(obj, step),
		FailFast: true,
	}); err != nil {
		if is, err := controller.IsObjectEnqueued(ctx); is {
			return err
		}
		readyStatus := aggregateNotReadyStatus(ctx, kubeClient, step.Objects)
		return stepError(step, "health check failed", fmt.Errorf("%w\n%s", err, readyStatus))
	}
	return nil
}

// trackPartialApply records the partial apply results returned by a failed
// ApplyAllStaged call in the inventory, so that a failed step never orphans
// the objects applied before the in-step failure. The merged inventory is
// set on the object status, which is persisted by the deferred finalizeStatus.
// It returns the apply error, joined with the inventory error if any.
func trackPartialApply(obj *fluxcdv1.ResourceSet,
	oldInventory, newInventory *fluxcdv1.ResourceInventory,
	changeSet *ssa.ChangeSet, applyErr error) error {
	if changeSet == nil || len(changeSet.Entries) == 0 {
		return applyErr
	}
	if err := inventory.AddChangeSet(newInventory, changeSet); err != nil {
		return errors.Join(applyErr, err)
	}
	obj.Status.Inventory = inventory.Merge(oldInventory, newInventory)
	return applyErr
}

// stepError wraps the given error with the step name and action for named
// steps. For the anonymous step of steps-less ResourceSets the error is
// returned unchanged so the legacy error messages stay identical.
func stepError(step builder.StepBuildResult, action string, err error) error {
	if step.Name == "" {
		return err
	}
	return fmt.Errorf("step %q %s: %w", step.Name, action, err)
}

// stepTimeout returns the health check timeout carried by the given build
// step, falling back to the ResourceSet reconciliation timeout when the
// step does not set one.
func stepTimeout(obj *fluxcdv1.ResourceSet, step builder.StepBuildResult) time.Duration {
	if step.Timeout != nil {
		return step.Timeout.Duration
	}
	return obj.GetTimeout()
}

// deleteFailedJobs deletes the failed Jobs of the given step which are
// annotated with the recreateOnFailure annotation set to enabled, so that
// the subsequent server-side apply can recreate them from scratch.
// Only Jobs carrying this ResourceSet's owner labels are deleted, the
// deletion uses foreground propagation with a UID precondition and waits
// for the Jobs to be removed from the cluster before returning.
func (r *ResourceSetReconciler) deleteFailedJobs(ctx context.Context,
	kubeClient client.Client,
	rm *ssa.ResourceManager,
	obj *fluxcdv1.ResourceSet,
	step builder.StepBuildResult) error {
	log := ctrl.LoggerFrom(ctx)
	ownerSelector := labels.SelectorFromSet(rm.GetOwnerLabels(obj.Name, obj.Namespace))

	var deletedJobs []*unstructured.Unstructured
	for _, desired := range step.Objects {
		if desired.GetAPIVersion() != "batch/v1" || desired.GetKind() != "Job" ||
			desired.GetAnnotations()[fluxcdv1.RecreateOnFailureAnnotation] != fluxcdv1.EnabledValue {
			continue
		}

		// Read the Job from the cluster to inspect its status.
		job := &unstructured.Unstructured{}
		job.SetGroupVersionKind(desired.GroupVersionKind())
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(desired), job); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed to read Job %s status: %w", ssautil.FmtUnstructured(desired), err)
		}

		// Skip the Jobs which have not failed.
		if !isJobFailed(job) {
			continue
		}

		// Never delete a Job owned by another manager.
		if !ownerSelector.Matches(labels.Set(job.GetLabels())) {
			continue
		}

		// Delete the failed Job with foreground propagation so that its
		// pods are removed as well, using the UID precondition to avoid
		// deleting a Job recreated by an external actor in the meantime.
		jobUID := job.GetUID()
		if err := kubeClient.Delete(ctx, job,
			client.PropagationPolicy(metav1.DeletePropagationForeground),
			client.Preconditions{UID: &jobUID}); err != nil {
			return fmt.Errorf("failed to delete failed Job %s: %w", ssautil.FmtUnstructured(desired), err)
		}
		log.Info("Failed Job deleted for recreation", "job", ssautil.FmtUnstructured(desired))
		deletedJobs = append(deletedJobs, job)
	}

	// Wait for the deleted Jobs to be removed from the cluster
	// so that the subsequent server-side apply can recreate them.
	if len(deletedJobs) > 0 {
		if err := rm.WaitForTermination(deletedJobs, ssa.DefaultWaitOptions()); err != nil {
			return fmt.Errorf("failed to wait for Jobs termination: %w", err)
		}
	}

	return nil
}

// isJobFailed returns true if the given Job has failed according to its
// kstatus computed status. A Job with a malformed status which fails the
// kstatus computation is treated as not failed.
func isJobFailed(job *unstructured.Unstructured) bool {
	res, err := status.Compute(job)
	return err == nil && res.Status == status.FailedStatus
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

// makeImpersonator creates an impersonator for the ResourceSet.
// It configures service account impersonation and custom health check readers.
func (r *ResourceSetReconciler) makeImpersonator(obj *fluxcdv1.ResourceSet) (*runtimeClient.Impersonator, error) {
	var impersonatorOpts []runtimeClient.ImpersonatorOption

	// Configure service account for impersonation.
	if r.DefaultServiceAccount != "" || obj.Spec.ServiceAccountName != "" {
		impersonatorOpts = append(impersonatorOpts,
			runtimeClient.WithServiceAccount(r.DefaultServiceAccount, obj.Spec.ServiceAccountName, obj.GetNamespace()))
	}

	// Configure the kstatus poller with custom health checks for
	// Flux Operator owned resources.
	if r.ClusterReader != nil {
		kinds := []string{fluxcdv1.FluxInstanceKind, fluxcdv1.ResourceSetKind, fluxcdv1.ResourceSetInputProviderKind}
		healthChecks := make([]kustomize.CustomHealthCheck, 0, len(kinds))
		for _, kind := range kinds {
			healthChecks = append(healthChecks, kustomize.CustomHealthCheck{
				APIVersion: fluxcdv1.GroupVersion.String(),
				Kind:       kind,
				HealthCheckExpressions: kustomize.HealthCheckExpressions{
					Current: fluxcdv1.HealthCheckExpr,
				},
			})
		}

		statusReader, err := cel.NewStatusReader(healthChecks)
		if err != nil {
			return nil, fmt.Errorf("failed to create custom health check readers: %w", err)
		}

		impersonatorOpts = append(impersonatorOpts,
			runtimeClient.WithPolling(r.ClusterReader, statusReader),
		)
	}

	return runtimeClient.NewImpersonator(r.Client, impersonatorOpts...), nil
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
