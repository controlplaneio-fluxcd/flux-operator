// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/fluxcd/pkg/ssa"
	ssadiff "github.com/fluxcd/pkg/ssa/jsondiff"
	"github.com/fluxcd/pkg/ssa/normalize"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/wI2L/jsondiff"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Apply parses the YAML manifest and creates or updates the Kubernetes objects using server-side apply.
// If any of the Kubernetes objects are managed by Flux, it will return an error unless overwrite is set to true.
func (k *Client) Apply(ctx context.Context, manifest string, overwrite bool) (string, error) {
	objects, err := ssautil.ReadObjects(strings.NewReader(manifest))
	if err != nil {
		return "", fmt.Errorf("unable to parse YAML manifest: %w", err)
	}

	if len(objects) == 0 {
		return "", fmt.Errorf("no Kubernetes objects found in manifest")
	}

	if !overwrite {
		for _, object := range objects {
			if k.IsManagedByFlux(ctx, object.GroupVersionKind(), object.GetName(), object.GetNamespace()) {
				return "", fmt.Errorf("%s/%s is managed by Flux",
					object.GetKind(), object.GetName())
			}
		}
	}

	err = normalize.UnstructuredList(objects)
	if err != nil {
		return "", fmt.Errorf("unable to normalize objects: %w", err)
	}

	opts := ssa.DefaultApplyOptions()
	opts.CustomStageKinds = map[schema.GroupKind]struct{}{
		{Group: "rbac.authorization.k8s.io", Kind: "Role"}: {},
	}
	changeSet, err := k.rm.ApplyAllStaged(ctx, objects, opts)
	if err != nil {
		return "", fmt.Errorf("unable to apply objects: %w", err)
	}

	return changeSet.String(), nil
}

// PatchRequest identifies a Kubernetes resource and describes the raw patch to apply.
type PatchRequest struct {
	GVK         schema.GroupVersionKind
	Name        string
	Namespace   string
	Patch       string
	Type        string
	Subresource string
	DryRun      bool
	Overwrite   bool
}

// Patch applies a merge, JSON, or strategic merge patch and returns the resulting RFC 6902 diff.
func (k *Client) Patch(ctx context.Context, req PatchRequest) (string, error) {
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(req.GVK)
	key := ctrlclient.ObjectKey{Namespace: req.Namespace, Name: req.Name}
	isSecret := req.GVK.Group == "" && req.GVK.Version == "v1" && req.GVK.Kind == "Secret"
	getErr := k.Client.Get(ctx, key, live)
	if err := ctrlclient.IgnoreNotFound(getErr); err != nil {
		if isSecret {
			return "", fmt.Errorf("unable to read %s/%s/%s error: %s", req.GVK.Kind, req.Namespace, req.Name, apiErrorReason(err))
		}
		return "", fmt.Errorf("unable to read %s/%s/%s error: %w", req.GVK.Kind, req.Namespace, req.Name, err)
	}
	if apierrors.IsNotFound(getErr) {
		return "", fmt.Errorf("%s/%s/%s not found", req.GVK.Kind, req.Namespace, req.Name)
	}

	if !req.Overwrite && k.IsManagedByFlux(ctx, req.GVK, req.Name, req.Namespace) {
		return "", fmt.Errorf("%s/%s is managed by Flux, set overwrite to patch it", req.GVK.Kind, req.Name)
	}

	body, err := yaml.YAMLToJSON([]byte(req.Patch))
	if err != nil {
		return "", fmt.Errorf("unable to parse patch: %w", err)
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return "", fmt.Errorf("unable to parse patch: %w", err)
	}

	var patchType types.PatchType
	switch req.Type {
	case "", "merge":
		patchType = types.MergePatchType
		if _, ok := document.(map[string]any); !ok {
			return "", fmt.Errorf("patch must be a YAML/JSON object")
		}
	case "json":
		patchType = types.JSONPatchType
		if _, ok := document.([]any); !ok {
			return "", fmt.Errorf("patch must be a list of JSON patch operations")
		}
	case "strategic":
		patchType = types.StrategicMergePatchType
		if _, ok := document.(map[string]any); !ok {
			return "", fmt.Errorf("patch must be a YAML/JSON object")
		}
	default:
		return "", fmt.Errorf("type must be merge, json or strategic")
	}

	patched := live.DeepCopy()
	rawPatch := ctrlclient.RawPatch(patchType, body)
	if req.Subresource == "status" {
		opts := []ctrlclient.SubResourcePatchOption{ctrlclient.FieldOwner(patchFieldManager)}
		if req.DryRun {
			opts = append(opts, ctrlclient.DryRunAll)
		}
		err = k.Client.SubResource("status").Patch(ctx, patched, rawPatch, opts...)
	} else {
		opts := []ctrlclient.PatchOption{ctrlclient.FieldOwner(patchFieldManager)}
		if req.DryRun {
			opts = append(opts, ctrlclient.DryRunAll)
		}
		err = k.Client.Patch(ctx, patched, rawPatch, opts...)
	}
	if err != nil {
		if isSecret {
			return "", fmt.Errorf("unable to patch %s/%s/%s error: %s", req.GVK.Kind, req.Namespace, req.Name, apiErrorReason(err))
		}
		return "", fmt.Errorf("unable to patch %s/%s/%s error: %s", req.GVK.Kind, req.Namespace, req.Name,
			patchErrorMessage(err, patchType, live, body))
	}

	liveForDiff := live.DeepCopy()
	patchedForDiff := patched.DeepCopy()
	for _, object := range []*unstructured.Unstructured{liveForDiff, patchedForDiff} {
		unstructured.RemoveNestedField(object.Object, "metadata", "managedFields")
		unstructured.RemoveNestedField(object.Object, "metadata", "resourceVersion")
		unstructured.RemoveNestedField(object.Object, "metadata", "generation")
	}
	if err := normalize.DryRunUnstructured(patchedForDiff); err != nil {
		return "", fmt.Errorf("unable to normalize patched object: %w", err)
	}
	patch, err := jsondiff.Compare(liveForDiff.Object, patchedForDiff.Object, jsondiff.Rationalize())
	if err != nil {
		return "", fmt.Errorf("unable to diff patched object: %w", err)
	}
	if isSecret {
		patch = ssadiff.MaskSecretPatchData(patch)
	}

	state := diffStatePatched
	detail := ""
	if len(patch) == 0 {
		state = diffStateUnchanged
	}
	if req.DryRun {
		if state == diffStatePatched {
			detail = "dry-run, nothing was changed"
		} else {
			detail = "dry-run"
		}
	}
	var builder strings.Builder
	if err := renderDiffObject(&builder, diffObjectResult{
		Subject: ssautil.FmtUnstructured(live),
		State:   state,
		Detail:  detail,
		Patch:   patch,
	}); err != nil {
		return "", err
	}
	return builder.String(), nil
}

// echoedObjectPattern matches the whole object the API server quotes in decoding errors.
var echoedObjectPattern = regexp.MustCompile(`Invalid value: "\{(?:[^"\\]|\\.)*\}"`)

// patchErrorMessage trims the API server error for a rejected patch: the echoed object is
// dropped from decoding errors, and JSON patch failures, which the server reports without
// detail, are explained by applying the patch locally to the live object.
func patchErrorMessage(err error, patchType types.PatchType, live *unstructured.Unstructured, patch []byte) string {
	message := echoedObjectPattern.ReplaceAllString(err.Error(), "Invalid value")
	message = strings.TrimPrefix(strings.TrimSpace(message), `"" is invalid: `)
	if patchType == types.JSONPatchType && apierrors.IsInvalid(err) {
		message += ": " + jsonPatchFailure(live, patch)
	}
	return message
}

// jsonPatchFailure applies a JSON patch to the live object and returns the failure detail.
// The result only explains the API server rejection and never decides the outcome.
func jsonPatchFailure(live *unstructured.Unstructured, patch []byte) string {
	decoded, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return err.Error()
	}
	document, err := json.Marshal(live.Object)
	if err != nil {
		return err.Error()
	}
	if _, err := decoded.Apply(document); err != nil {
		return err.Error()
	}
	return "a JSON patch operation could not be applied, read the object and check the paths"
}

// GetFluxOwner returns the Flux owner encoded in the resource's ownership labels.
func (k *Client) GetFluxOwner(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string) (*DiffOwnerRef, bool) {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return nil, false
	}

	return fluxOwnerFromLabels(resource.GetLabels())
}

// IsManagedByFlux checks if a Kubernetes resource is managed by Flux by inspecting specific Flux-related labels.
func (k *Client) IsManagedByFlux(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string) bool {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)
	if err := k.Client.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: name}, resource); err != nil {
		return false
	}
	for _, group := range []string{
		fluxcdv1.GroupVersion.Group,
		fluxcdv1.GroupOwnerLabelResourceSet,
		fluxcdv1.FluxKustomizeGroup,
		fluxcdv1.FluxHelmGroup,
	} {
		if resource.GetLabels()[group+"/namespace"] != "" {
			return true
		}
	}
	return false
}

// fluxOwnerFromLabels returns the first recognized Flux owner reference from labels.
func fluxOwnerFromLabels(labels map[string]string) (*DiffOwnerRef, bool) {
	owners := []struct {
		group string
		kind  string
	}{
		{fluxcdv1.FluxKustomizeGroup, fluxcdv1.FluxKustomizationKind},
		{fluxcdv1.GroupOwnerLabelResourceSet, fluxcdv1.ResourceSetKind},
		{fluxcdv1.FluxHelmGroup, fluxcdv1.FluxHelmReleaseKind},
		{fluxcdv1.GroupVersion.Group, "FluxInstance"},
	}

	for _, owner := range owners {
		name := labels[owner.group+"/name"]
		namespace := labels[owner.group+"/namespace"]
		if name != "" && namespace != "" {
			return &DiffOwnerRef{Kind: owner.kind, Name: name, Namespace: namespace}, true
		}
	}

	return nil, false
}

// Annotate sets annotations on a Kubernetes resource identified by GroupVersionKind, name, and namespace.
func (k *Client) Annotate(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, keys []string, val string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := ctrlclient.MergeFrom(resource.DeepCopy())

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	for _, key := range keys {
		annotations[key] = val
		resource.SetAnnotations(annotations)
	}

	if err := k.Client.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// Delete deletes a Kubernetes resource identified by GroupVersionKind, name, and namespace.
func (k *Client) Delete(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(gvk)
	resource.SetName(name)
	resource.SetNamespace(namespace)

	if err := k.Client.Delete(ctx, resource); err != nil {
		return fmt.Errorf("unable to delete %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}

// ToggleSuspension toggles the suspension of a Flux resource by updating the spec.suspend field.
func (k *Client) ToggleSuspension(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, suspend bool) error {
	if strings.EqualFold(gvk.Group, fluxcdv1.GroupVersion.Group) {
		val := fluxcdv1.EnabledValue
		if suspend {
			val = fluxcdv1.DisabledValue
		}
		return k.Annotate(ctx,
			gvk,
			name,
			namespace,
			[]string{fluxcdv1.ReconcileAnnotation},
			val)
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)

	objectKey := ctrlclient.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := k.Client.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := ctrlclient.MergeFrom(resource.DeepCopy())

	if suspend {
		err := unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend")
		if err != nil {
			return fmt.Errorf("unable to set suspend field: %w", err)
		}
	} else {
		unstructured.RemoveNestedField(resource.Object, "spec", "suspend")
	}

	if err := k.Client.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to patch %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}
