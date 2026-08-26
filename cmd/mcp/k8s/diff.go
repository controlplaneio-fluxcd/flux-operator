// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	objectutil "github.com/fluxcd/cli-utils/pkg/object"
	"github.com/fluxcd/pkg/ssa"
	ssadiff "github.com/fluxcd/pkg/ssa/jsondiff"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/wI2L/jsondiff"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

// diffExecution retains non-rendered state needed for dependent-resource recovery and pruning.
type diffExecution struct {
	result       diffObjectResult
	object       *unstructured.Unstructured
	live         *unstructured.Unstructured
	err          error
	preGetAbsent bool
}

// readDiffObjects mirrors ssautil.ReadObjects but preserves generateName-only
// objects so Diff can render the required per-object validation error.
func readDiffObjects(reader io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(reader, 2048)
	objects := make([]*unstructured.Unstructured, 0)
	appendObject := func(resource *unstructured.Unstructured) {
		if resource.GetAPIVersion() == "" || resource.GetKind() == "" {
			return
		}
		isGenerateNameObject := resource.GetGenerateName() != ""
		if (ssautil.IsKubernetesObject(resource) || isGenerateNameObject) && !ssautil.IsKustomization(resource) {
			objects = append(objects, resource)
		}
	}

	for {
		var document any
		if err := decoder.Decode(&document); err != nil {
			if err == io.EOF {
				return objects, nil
			}
			return objects, err
		}
		objectMap, ok := document.(map[string]any)
		if !ok {
			continue
		}
		object := &unstructured.Unstructured{Object: objectMap}
		if object.IsList() {
			if err := object.EachListItem(func(item runtime.Object) error {
				appendObject(item.(*unstructured.Unstructured))
				return nil
			}); err != nil {
				return objects, err
			}
			continue
		}
		appendObject(object)
	}
}

// Diff previews a Kubernetes manifest using server-side apply dry-run with the selected Flux owner.
func (k *Client) Diff(ctx context.Context, req DiffRequest) (string, error) {
	objects, err := readDiffObjects(strings.NewReader(req.Manifest))
	if err != nil {
		return "", fmt.Errorf("unable to parse YAML manifest: %w", err)
	}
	if len(objects) == 0 {
		return "", fmt.Errorf("no Kubernetes objects found in manifest")
	}

	owner, err := k.resolveDiffOwner(ctx, req)
	if err != nil {
		return "", err
	}

	rm := k.rm
	fieldManager := "kubectl-flux-mcp"
	diffOptions := ssa.DefaultDiffOptions()
	if owner != nil {
		fieldManager = owner.ssaOwner.Field
		diffOptions = owner.diffOptions
		rm = newResourceManagerWithOwner(k.Client, k.poller, owner.ssaOwner)
	}
	if rm == nil {
		rm = newResourceManagerWithOwner(k.Client, k.poller, ssa.Owner{
			Field: fieldManager,
			Group: fluxcdv1.GroupVersion.Group,
		})
	}

	if err := prepareDiffObjects(ctx, k.Client, &objects, owner, rm); err != nil {
		return "", fmt.Errorf("unable to prepare objects: %w", err)
	}
	sopsKeySets := sanitizeSOPSObjects(objects)
	if err := normalize.UnstructuredList(objects); err != nil {
		return "", fmt.Errorf("unable to normalize objects: %w", err)
	}

	executions := make([]diffExecution, 0, len(objects))
	pruneManifestObjects := make([]*unstructured.Unstructured, 0, len(objects))
	for _, desired := range objects {
		execution := diffExecution{
			object: desired,
			result: diffObjectResult{Subject: ssautil.FmtUnstructured(desired)},
		}
		if hook, ok := helmHookValue(owner, desired); ok {
			execution.result.State = diffStateSkipped
			execution.result.Detail = "helm.sh/hook: " + hook
			executions = append(executions, execution)
			continue
		}
		pruneManifestObjects = append(pruneManifestObjects, desired)
		if desired.GetName() == "" {
			if desired.GetGenerateName() != "" {
				execution.result.Subject = generatedSubject(desired)
				execution.err = fmt.Errorf("metadata.generateName is not supported without metadata.name")
			} else {
				execution.err = fmt.Errorf("metadata.name is required")
			}
			execution.result.State = diffStateError
			execution.result.Detail = execution.err.Error()
			executions = append(executions, execution)
			continue
		}

		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(desired.GroupVersionKind())
		getErr := k.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(desired), live)
		present := getErr == nil
		execution.preGetAbsent = apierrors.IsNotFound(getErr) || isNoMatchError(getErr)
		switch {
		case present:
			execution.live = live
			execution.result.Hint = managedByHint(live, owner)
		case apierrors.IsNotFound(getErr):
			// Absence is an expected input to server-side apply.
		default:
			execution.err = getErr
			execution.result.State = diffStateError
			execution.result.Detail = diffErrorMessage(desired, getErr, true)
			executions = append(executions, execution)
			continue
		}
		if present {
			applyHelmMetadataForLiveCRD(desired, live, owner)
		}

		if desiredKeys, encrypted := sopsKeySets[desired]; encrypted {
			if !present {
				execution.result.State = diffStateCreate
			} else if equalStringSets(desiredKeys, objectDataKeys(live)) {
				execution.result.State = diffStateUnchanged
			} else {
				execution.result.State = diffStateUpdate
				execution.result.Detail = "sops encrypted: keys compared only"
			}
			executions = append(executions, execution)
			continue
		}

		entry, liveDiff, merged, diffErr := rm.Diff(ctx, desired, diffOptions)
		if diffErr != nil {
			execution.err = diffErr
			execution.result.State = diffStateError
			execution.result.Detail = diffErrorMessage(desired, diffErr, false)
			executions = append(executions, execution)
			continue
		}

		switch entry.Action {
		case ssa.ConfiguredAction:
			patch, err := diffPatch(liveDiff, merged, diffOptions.DriftIgnoreRules)
			if err != nil {
				execution.err = err
				execution.result.State = diffStateError
				execution.result.Detail = err.Error()
			} else if len(patch) == 0 {
				execution.result.State = diffStateUnchanged
			} else {
				execution.result.State = diffStateUpdate
				execution.result.Patch = patch
			}
		case ssa.UnchangedAction:
			execution.result.State = diffStateUnchanged
		case ssa.SkippedAction:
			execution.result.State = diffStateSkipped
			execution.result.Detail = skippedBy(desired, live, present, diffOptions)
		case ssa.CreatedAction:
			if present {
				execution.result.State = diffStateRecreate
				execution.result.Detail = "forced, not validated"
			} else {
				execution.result.State = diffStateCreate
			}
		default:
			execution.err = fmt.Errorf("unexpected dry-run action %q", entry.Action)
			execution.result.State = diffStateError
			execution.result.Detail = execution.err.Error()
		}
		executions = append(executions, execution)
	}

	result := diffResult{
		FieldManager: fieldManager,
		Objects:      make([]diffObjectResult, 0, len(executions)),
	}
	if owner != nil {
		result.Owner = owner.ref
		result.PruneEnabled = owner.prune
		result.FutureSuspend = owner.futureSuspend
		result.LiveSuspend = owner.liveSuspend
		result.Warnings = append(result.Warnings, owner.warnings...)
	}
	recoverDependentCreates(executions, &result.Warnings)
	hasErrors := false
	for _, execution := range executions {
		result.Objects = append(result.Objects, execution.result)
		if execution.result.State == diffStateError {
			hasErrors = true
		}
	}

	if owner != nil && owner.prune && !hasErrors {
		pruneObjects, pruneStatus := k.diffPrune(ctx, owner, rm, pruneManifestObjects)
		result.PruneObjects = pruneObjects
		result.PruneStatus = pruneStatus
	}
	return renderDiff(result)
}

// helmHookValue returns a Helm hook annotation only for HelmRelease-owned objects.
func helmHookValue(owner *diffOwner, object *unstructured.Unstructured) (string, bool) {
	if owner == nil || owner.ref.Kind != fluxcdv1.FluxHelmReleaseKind {
		return "", false
	}
	value, found := object.GetAnnotations()["helm.sh/hook"]
	return value, found
}

// sanitizeSOPSObjects removes encrypted payloads and returns their desired key sets.
func sanitizeSOPSObjects(objects []*unstructured.Unstructured) map[*unstructured.Unstructured]map[string]struct{} {
	result := make(map[*unstructured.Unstructured]map[string]struct{})
	for _, object := range objects {
		if _, found := object.Object["sops"]; !found {
			continue
		}
		result[object] = objectDataKeys(object)
		delete(object.Object, "sops")
		delete(object.Object, "data")
		delete(object.Object, "stringData")
	}
	return result
}

// objectDataKeys returns the union of top-level data and stringData keys.
func objectDataKeys(object *unstructured.Unstructured) map[string]struct{} {
	result := make(map[string]struct{})
	for _, field := range []string{"data", "stringData"} {
		if values, found, _ := unstructured.NestedMap(object.Object, field); found {
			for key := range values {
				result[key] = struct{}{}
			}
		}
	}
	return result
}

// equalStringSets reports whether two string sets contain identical keys.
func equalStringSets(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if _, found := right[key]; !found {
			return false
		}
	}
	return true
}

// diffPatch computes spec and add-or-replace metadata JSON patch operations.
func diffPatch(live, merged *unstructured.Unstructured, ignoreRules []ssadiff.IgnoreRule) (jsondiff.Patch, error) {
	if live == nil || merged == nil {
		return nil, fmt.Errorf("dry-run returned no objects for configured change")
	}
	if err := normalize.DryRunUnstructured(merged); err != nil {
		return nil, fmt.Errorf("normalizing dry-run object: %w", err)
	}
	filteredLive := live.DeepCopy()
	filteredMerged := merged.DeepCopy()
	if err := removeDiffIgnoredFields(merged, []*unstructured.Unstructured{filteredLive, filteredMerged}, ignoreRules); err != nil {
		return nil, err
	}
	metadataPatch, err := diffUnstructuredMetadata(filteredLive, filteredMerged, nil, jsondiff.Rationalize())
	if err != nil {
		return nil, err
	}
	resourcePatch, err := ssadiff.DiffUnstructured(filteredLive, filteredMerged, jsondiff.Rationalize())
	if err != nil {
		return nil, err
	}
	return append(metadataPatch, resourcePatch...), nil
}

// diffUnstructuredMetadata mirrors pkg/ssa's metadata diff while excluding remove operations.
func diffUnstructuredMetadata(live, merged *unstructured.Unstructured, ignoreRules []ssadiff.IgnoreRule,
	opts ...jsondiff.Option) (jsondiff.Patch, error) {
	filteredLive := live.DeepCopy()
	filteredMerged := merged.DeepCopy()
	if err := removeDiffIgnoredFields(merged, []*unstructured.Unstructured{filteredLive, filteredMerged}, ignoreRules); err != nil {
		return nil, err
	}
	patch, err := jsondiff.Compare(metadataOnly(filteredLive).Object, metadataOnly(filteredMerged).Object, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to compare annotations and labels of objects: %w", err)
	}
	filtered := make(jsondiff.Patch, 0, len(patch))
	for _, operation := range patch {
		if operation.Type == jsondiff.OperationAdd || operation.Type == jsondiff.OperationReplace {
			filtered = append(filtered, operation)
		}
	}
	return filtered, nil
}

// removeDiffIgnoredFields applies rules selected against matchObject to every object.
func removeDiffIgnoredFields(matchObject *unstructured.Unstructured, objects []*unstructured.Unstructured,
	rules []ssadiff.IgnoreRule) error {
	if len(rules) == 0 {
		return nil
	}
	compiled, err := ssadiff.CompileIgnoreRules(rules)
	if err != nil {
		return err
	}
	var paths []string
	for selector, rulePaths := range compiled {
		if selector.MatchUnstructured(matchObject) {
			paths = append(paths, rulePaths...)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	patch := ssadiff.GenerateRemovePatch(paths...)
	for _, object := range objects {
		if err := ssadiff.ApplyPatchToUnstructured(object, patch); err != nil {
			return err
		}
	}
	return nil
}

// metadataOnly copies name, annotations and labels into a fresh object.
func metadataOnly(source *unstructured.Unstructured) *unstructured.Unstructured {
	result := &unstructured.Unstructured{Object: make(map[string]any)}
	for _, field := range []string{"name", "annotations", "labels"} {
		if value, found, _ := unstructured.NestedFieldCopy(source.Object, "metadata", field); found {
			_ = unstructured.SetNestedField(result.Object, value, "metadata", field)
		}
	}
	return result
}

// generatedSubject formats an object that has generateName but no SSA-compatible name.
func generatedSubject(object *unstructured.Unstructured) string {
	if object.GetNamespace() == "" {
		return fmt.Sprintf("%s/%s*", object.GetKind(), object.GetGenerateName())
	}
	return fmt.Sprintf("%s/%s/%s*", object.GetKind(), object.GetNamespace(), object.GetGenerateName())
}

// diffErrorMessage applies permission guidance and Secret-safe API error redaction.
func diffErrorMessage(object *unstructured.Unstructured, err error, get bool) string {
	if apierrors.IsForbidden(err) {
		return fmt.Sprintf("dry-run requires get and patch permission on %s", ssautil.FmtUnstructured(object))
	}
	if object.GetAPIVersion() == "v1" && object.GetKind() == "Secret" {
		for current := err; current != nil; current = errors.Unwrap(current) {
			if reason := apierrors.ReasonForError(current); reason != metav1.StatusReasonUnknown && reason != "" {
				return string(reason)
			}
		}
		return "API error"
	}
	if get {
		return fmt.Sprintf("unable to get live object: %v", err)
	}
	return err.Error()
}

// skippedBy identifies the selector annotation or label responsible for a skipped dry-run.
func skippedBy(desired, live *unstructured.Unstructured, present bool, opts ssa.DiffOptions) string {
	selectors := []map[string]string{opts.Exclusions}
	if present {
		selectors = append(selectors, opts.IfNotPresentSelector)
	}
	for _, selector := range selectors {
		for key, expected := range selector {
			for _, candidate := range []*unstructured.Unstructured{desired, live} {
				if candidate == nil {
					continue
				}
				if value := candidate.GetAnnotations()[key]; value != "" && strings.EqualFold(value, expected) {
					return key + ": " + value
				}
				if value := candidate.GetLabels()[key]; value != "" && strings.EqualFold(value, expected) {
					return key + ": " + value
				}
			}
		}
	}
	return "excluded"
}

// managedByHint returns a live Flux owner when it differs from the selected owner.
func managedByHint(live *unstructured.Unstructured, selected *diffOwner) *DiffOwnerRef {
	current, ok := fluxOwnerFromLabels(live.GetLabels())
	if !ok {
		return nil
	}
	if selected != nil && *current == *selected.ref {
		return nil
	}
	return current
}

// recoverDependentCreates marks absent objects which cannot be dry-run before their Namespace or CRD exists.
func recoverDependentCreates(executions []diffExecution, warnings *[]string) {
	for i := range executions {
		execution := &executions[i]
		if execution.result.State != diffStateError || execution.err == nil || !execution.preGetAbsent {
			continue
		}

		namespace := execution.object.GetNamespace()
		if isMissingNamespaceError(execution.err, namespace) {
			execution.err = nil
			execution.result.State = diffStateCreate
			execution.result.Detail = fmt.Sprintf("not validated: namespace %s does not exist yet", namespace)
			appendDiffWarning(warnings, fmt.Sprintf("namespace %s does not exist in the cluster, objects in it are not validated", namespace))
			continue
		}

		if isNoMatchError(execution.err) {
			groupKind := execution.object.GroupVersionKind().Group + "/" + execution.object.GetKind()
			execution.err = nil
			execution.result.State = diffStateCreate
			execution.result.Detail = fmt.Sprintf("not validated: CRD for %s does not exist yet", groupKind)
			appendDiffWarning(warnings, fmt.Sprintf("CRD for %s does not exist in the cluster, objects of that kind are not validated", groupKind))
		}
	}
}

// isMissingNamespaceError checks for the structured NotFound response returned for an absent namespace.
func isMissingNamespaceError(err error, namespace string) bool {
	if namespace == "" {
		return false
	}
	expectedMessage := fmt.Sprintf("namespaces %q not found", namespace)
	for current := err; current != nil; current = errors.Unwrap(current) {
		if !apierrors.IsNotFound(current) {
			continue
		}
		statusError, ok := current.(apierrors.APIStatus)
		if !ok {
			continue
		}
		status := statusError.Status()
		if status.Details != nil && status.Details.Kind == "namespaces" &&
			(status.Details.Name == namespace || status.Message == expectedMessage) {
			return true
		}
	}
	return false
}

// isNoMatchError checks an error chain for a REST mapping failure.
func isNoMatchError(err error) bool {
	for current := err; current != nil; current = errors.Unwrap(current) {
		if meta.IsNoMatchError(current) {
			return true
		}
	}
	return false
}

// diffPrune computes and live-filters owner inventory entries absent from the manifest.
func (k *Client) diffPrune(ctx context.Context, owner *diffOwner, rm *ssa.ResourceManager,
	objects []*unstructured.Unstructured) ([]diffObjectResult, string) {
	if owner.live == nil {
		return nil, "prune: skipped (owner not in cluster)"
	}
	inventoryValue, found, err := unstructured.NestedFieldCopy(owner.live.Object, "status", "inventory")
	if err != nil || !found || inventoryValue == nil {
		return nil, "prune: unavailable"
	}
	inventoryMap, ok := inventoryValue.(map[string]any)
	if !ok {
		return nil, "prune: unavailable"
	}
	liveInventory := &fluxcdv1.ResourceInventory{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(inventoryMap, liveInventory); err != nil {
		return nil, "prune: unavailable"
	}

	targetInventory := inventory.New()
	for _, item := range objects {
		targetInventory.Entries = append(targetInventory.Entries, fluxcdv1.ResourceRef{
			ID:      objectutil.UnstructuredToObjMetadata(item).String(),
			Version: item.GroupVersionKind().Version,
		})
	}
	candidates, err := inventory.Diff(liveInventory, targetInventory)
	if err != nil {
		return nil, "prune: unavailable"
	}

	results := make([]diffObjectResult, 0, len(candidates))
	ownerLabels := rm.GetOwnerLabels(owner.ref.Name, owner.ref.Namespace)
	for _, candidate := range candidates {
		metadata := &metav1.PartialObjectMetadata{}
		metadata.SetGroupVersionKind(candidate.GroupVersionKind())
		err := k.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(candidate), metadata)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			results = append(results, diffObjectResult{
				Subject: ssautil.FmtUnstructured(candidate),
				State:   diffStateError,
				Detail:  diffErrorMessage(candidate, err, true),
			})
			continue
		}
		if !hasOwnerLabels(metadata.GetLabels(), ownerLabels) || pruneDisabled(metadata, owner.ref.Kind) {
			continue
		}
		results = append(results, diffObjectResult{Subject: ssautil.FmtUnstructured(candidate), State: diffStateDelete})
	}
	return results, ""
}

// hasOwnerLabels checks that every expected owner label matches the live object.
func hasOwnerLabels(labels, expected map[string]string) bool {
	for key, value := range expected {
		if labels[key] != value {
			return false
		}
	}
	return true
}

// pruneDisabled applies controller-specific garbage-collection exclusions.
func pruneDisabled(object *metav1.PartialObjectMetadata, ownerKind string) bool {
	annotations := object.GetAnnotations()
	switch ownerKind {
	case fluxcdv1.FluxKustomizationKind:
		return strings.EqualFold(annotations[fluxcdv1.FluxKustomizeGroup+"/prune"], "disabled") ||
			strings.EqualFold(annotations[fluxcdv1.FluxKustomizeGroup+"/reconcile"], "disabled") ||
			strings.EqualFold(annotations[fluxcdv1.FluxKustomizeGroup+"/ssa"], "ignore")
	case fluxcdv1.ResourceSetKind:
		return strings.EqualFold(annotations[fluxcdv1.PruneAnnotation], fluxcdv1.DisabledValue)
	case fluxcdv1.FluxHelmReleaseKind:
		return object.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" ||
			strings.EqualFold(annotations["helm.sh/resource-policy"], "keep")
	default:
		return false
	}
}
