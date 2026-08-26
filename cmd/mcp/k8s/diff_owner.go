// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	apikustomize "github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/kustomize"
	"github.com/fluxcd/pkg/ssa"
	ssadiff "github.com/fluxcd/pkg/ssa/jsondiff"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/copier"
)

// DiffOwnerRef identifies a live Flux object which applies a manifest.
type DiffOwnerRef struct {
	Kind      string
	Name      string
	Namespace string
}

// DiffRequest contains a manifest and optional Flux ownership information.
type DiffRequest struct {
	Manifest   string
	FluxObject string
	OwnerRef   *DiffOwnerRef
}

// diffOwner contains the resolved future and live state of a manifest owner.
type diffOwner struct {
	ref           *DiffOwnerRef
	object        *unstructured.Unstructured
	live          *unstructured.Unstructured
	ssaOwner      ssa.Owner
	diffOptions   ssa.DiffOptions
	prune         bool
	futureSuspend bool
	liveSuspend   bool
	warnings      []string
}

// resolveDiffOwner applies flux_object precedence and resolves the live owner used for inventory.
func (k *Client) resolveDiffOwner(ctx context.Context, req DiffRequest) (*diffOwner, error) {
	if strings.TrimSpace(req.FluxObject) != "" {
		objects, err := ssautil.ReadObjects(strings.NewReader(req.FluxObject))
		if err != nil {
			return nil, fmt.Errorf("invalid flux_object: %w", err)
		}
		if len(objects) != 1 {
			return nil, fmt.Errorf("invalid flux_object: expected exactly one document")
		}

		object := objects[0]
		if object.GetNamespace() == "" {
			return nil, fmt.Errorf("invalid flux_object: metadata.namespace is required; pass the definition as it exists after the parent build")
		}
		if object.GetName() == "" {
			return nil, fmt.Errorf("invalid flux_object: metadata.name is required")
		}
		if err := validateDiffOwner(object); err != nil {
			return nil, err
		}

		owner, err := newDiffOwner(object)
		if err != nil {
			return nil, err
		}
		live := &unstructured.Unstructured{}
		live.SetGroupVersionKind(object.GroupVersionKind())
		err = k.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(object), live)
		switch {
		case err == nil:
			owner.live = live
			owner.liveSuspend, _, _ = unstructured.NestedBool(live.Object, "spec", "suspend")
		case apierrors.IsNotFound(err):
			// A future owner may not exist until the parent Flux object applies it.
		default:
			return nil, fmt.Errorf("unable to read live owner: %w", err)
		}
		return owner, nil
	}

	if req.OwnerRef == nil {
		return nil, nil
	}
	if req.OwnerRef.Name == "" || req.OwnerRef.Namespace == "" {
		return nil, fmt.Errorf("invalid owner reference: kind, name and namespace are required")
	}
	gvk, err := ownerGVK(req.OwnerRef.Kind)
	if err != nil {
		return nil, err
	}
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(gvk)
	if err := k.Client.Get(ctx, ctrlclient.ObjectKey{Name: req.OwnerRef.Name, Namespace: req.OwnerRef.Namespace}, live); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("owner not found in the cluster: %s/%s/%s; if it is new, pass its definition in flux_object",
				req.OwnerRef.Kind, req.OwnerRef.Namespace, req.OwnerRef.Name)
		}
		return nil, fmt.Errorf("unable to read owner reference: %w", err)
	}
	if err := validateDiffOwner(live); err != nil {
		return nil, err
	}
	owner, err := newDiffOwner(live)
	if err != nil {
		return nil, err
	}
	owner.live = live.DeepCopy()
	owner.liveSuspend = owner.futureSuspend
	return owner, nil
}

// ownerGVK returns the canonical v1 GroupVersionKind for a supported owner.
func ownerGVK(kind string) (schema.GroupVersionKind, error) {
	switch kind {
	case fluxcdv1.FluxKustomizationKind:
		return schema.GroupVersionKind{Group: fluxcdv1.FluxKustomizeGroup, Version: "v1", Kind: kind}, nil
	case fluxcdv1.ResourceSetKind:
		return fluxcdv1.GroupVersion.WithKind(kind), nil
	case fluxcdv1.FluxHelmReleaseKind:
		return schema.GroupVersionKind{Group: fluxcdv1.FluxHelmGroup, Version: "v2", Kind: kind}, nil
	default:
		return schema.GroupVersionKind{}, fmt.Errorf("unsupported diff owner kind %q", kind)
	}
}

// validateDiffOwner enforces supported owner kinds and the remote-cluster guard.
func validateDiffOwner(object *unstructured.Unstructured) error {
	expected, err := ownerGVK(object.GetKind())
	if err != nil || object.GroupVersionKind() != expected {
		return fmt.Errorf("invalid flux_object: supported kinds are Kustomization, ResourceSet and HelmRelease")
	}
	if value, found, _ := unstructured.NestedFieldNoCopy(object.Object, "spec", "kubeConfig"); found && value != nil {
		return fmt.Errorf("diff not supported for owners targeting remote clusters")
	}
	return nil
}

// isEmptyNestedValue reports whether an optional unstructured field has no effective value.
func isEmptyNestedValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

// newDiffOwner derives the SSA owner, selectors, force setting and prune policy.
func newDiffOwner(object *unstructured.Unstructured) (*diffOwner, error) {
	owner := &diffOwner{
		ref: &DiffOwnerRef{
			Kind:      object.GetKind(),
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
		},
		object:      object.DeepCopy(),
		diffOptions: ssa.DefaultDiffOptions(),
	}
	owner.futureSuspend, _, _ = unstructured.NestedBool(object.Object, "spec", "suspend")

	switch object.GetKind() {
	case fluxcdv1.FluxKustomizationKind:
		owner.ssaOwner = ssa.Owner{Field: "kustomize-controller", Group: fluxcdv1.FluxKustomizeGroup}
		owner.diffOptions.Exclusions = map[string]string{
			fluxcdv1.FluxKustomizeGroup + "/reconcile": "disabled",
			fluxcdv1.FluxKustomizeGroup + "/ssa":       "ignore",
		}
		owner.diffOptions.IfNotPresentSelector = map[string]string{
			fluxcdv1.FluxKustomizeGroup + "/ssa": "IfNotPresent",
		}
		owner.diffOptions.Force, _, _ = unstructured.NestedBool(object.Object, "spec", "force")
		owner.diffOptions.ForceSelector = map[string]string{
			fluxcdv1.FluxKustomizeGroup + "/force": "enabled",
		}
		owner.prune, _, _ = unstructured.NestedBool(object.Object, "spec", "prune")
		rules, err := kustomizationIgnoreRules(object)
		if err != nil {
			return nil, err
		}
		owner.diffOptions.DriftIgnoreRules = append(rules, buildMetadataIgnoreRules(object)...)
	case fluxcdv1.ResourceSetKind:
		owner.ssaOwner = ssa.Owner{Field: "flux-operator", Group: fluxcdv1.GroupOwnerLabelResourceSet}
		owner.diffOptions.Force = strings.EqualFold(object.GetAnnotations()[fluxcdv1.ForceAnnotation], fluxcdv1.EnabledValue)
		owner.diffOptions.ForceSelector = map[string]string{fluxcdv1.ForceAnnotation: fluxcdv1.EnabledValue}
		owner.prune = true
	case fluxcdv1.FluxHelmReleaseKind:
		owner.ssaOwner = ssa.Owner{Field: "helm-controller", Group: fluxcdv1.FluxHelmGroup}
		owner.diffOptions.DriftIgnoreRules = []ssadiff.IgnoreRule{{
			Paths: []string{
				"/metadata/labels/helm.sh~1chart",
				"/spec/template/metadata/labels/helm.sh~1chart",
				"/spec/jobTemplate/spec/template/metadata/labels/helm.sh~1chart",
			},
		}}
		owner.prune = true
	default:
		return nil, fmt.Errorf("unsupported diff owner kind %q", object.GetKind())
	}
	return owner, nil
}

// buildMetadataIgnoreRules returns ignore rules for the labels and annotations
// that spec.buildMetadata adds at build time. These are not reproduced by the
// diff and are excluded from the comparison instead.
func buildMetadataIgnoreRules(object *unstructured.Unstructured) []ssadiff.IgnoreRule {
	options, _, _ := unstructured.NestedStringSlice(object.Object, "spec", "buildMetadata")
	var paths []string
	for _, option := range options {
		switch option {
		case "originAnnotations":
			paths = append(paths, "/metadata/annotations/config.kubernetes.io~1origin")
		case "transformerAnnotations":
			paths = append(paths, "/metadata/annotations/config.kubernetes.io~1transformations")
		case "managedByLabel":
			paths = append(paths, "/metadata/labels/app.kubernetes.io~1managed-by")
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return []ssadiff.IgnoreRule{{Paths: paths}}
}

// kustomizationIgnoreRules converts spec.ignore to the pkg/ssa selector model.
func kustomizationIgnoreRules(object *unstructured.Unstructured) ([]ssadiff.IgnoreRule, error) {
	rawRules, found, err := unstructured.NestedSlice(object.Object, "spec", "ignore")
	if err != nil || !found {
		return nil, err
	}
	rules := make([]ssadiff.IgnoreRule, 0, len(rawRules))
	for _, raw := range rawRules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Kustomization spec.ignore rule")
		}
		paths, _, err := unstructured.NestedStringSlice(ruleMap, "paths")
		if err != nil {
			return nil, fmt.Errorf("invalid Kustomization spec.ignore paths: %w", err)
		}
		rule := ssadiff.IgnoreRule{Paths: paths}
		if target, found, err := unstructured.NestedMap(ruleMap, "target"); err != nil {
			return nil, fmt.Errorf("invalid Kustomization spec.ignore target: %w", err)
		} else if found {
			rule.Selector = &ssadiff.Selector{
				Group:              nestedString(target, "group"),
				Version:            nestedString(target, "version"),
				Kind:               nestedString(target, "kind"),
				Name:               nestedString(target, "name"),
				Namespace:          nestedString(target, "namespace"),
				AnnotationSelector: nestedString(target, "annotationSelector"),
				LabelSelector:      nestedString(target, "labelSelector"),
			}
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// nestedString returns a string field from an unstructured map.
func nestedString(object map[string]any, field string) string {
	value, _, _ := unstructured.NestedString(object, field)
	return value
}

// prepareDiffObjects applies build-time transforms and metadata injected by the selected owner.
func prepareDiffObjects(ctx context.Context, kubeClient ctrlclient.Client, objects *[]*unstructured.Unstructured,
	owner *diffOwner, rm *ssa.ResourceManager) error {
	if owner == nil {
		return nil
	}

	if err := applyDeferredOwnerTransforms(ctx, kubeClient, objects, owner); err != nil {
		return err
	}

	labels, annotations, err := commonMetadata(owner.object)
	if err != nil {
		return err
	}
	if len(labels) > 0 || len(annotations) > 0 {
		ssautil.SetCommonMetadata(*objects, labels, annotations)
	}

	switch owner.ref.Kind {
	case fluxcdv1.FluxKustomizationKind, fluxcdv1.ResourceSetKind:
		rm.SetOwnerLabels(*objects, owner.ref.Name, owner.ref.Namespace)
	case fluxcdv1.FluxHelmReleaseKind:
		for _, object := range *objects {
			objectLabels := object.GetLabels()
			if objectLabels == nil {
				objectLabels = make(map[string]string)
			}
			objectLabels[fluxcdv1.FluxHelmGroup+"/name"] = owner.ref.Name
			objectLabels[fluxcdv1.FluxHelmGroup+"/namespace"] = owner.ref.Namespace
			object.SetLabels(objectLabels)
		}
		if err := applyHelmReleaseMetadata(kubeClient, *objects, owner.object); err != nil {
			return err
		}
	}
	return nil
}

// applyDeferredOwnerTransforms applies transforms that require the owner spec or cluster reads.
// ResourceSet checksumFrom and convertKubeConfigFrom remain intentionally unsupported.
func applyDeferredOwnerTransforms(ctx context.Context, kubeClient ctrlclient.Client,
	objects *[]*unstructured.Unstructured, owner *diffOwner) error {
	switch owner.ref.Kind {
	case fluxcdv1.FluxKustomizationKind:
		if value, found, _ := unstructured.NestedFieldNoCopy(owner.object.Object, "spec", "components"); found && !isEmptyNestedValue(value) {
			owner.warnings = append(owner.warnings, "spec.components not applied, diff may be incomplete")
		}

		resources, rebuilt, err := buildKustomizationResources(*objects, owner.object)
		if err != nil {
			return err
		}
		postBuild := hasPostBuild(owner.object)
		if !rebuilt && !postBuild {
			return nil
		}
		if !rebuilt {
			resources, err = objectsResMap(*objects)
			if err != nil {
				return err
			}
		}
		if postBuild {
			skip, err := warnForMissingSubstituteFrom(ctx, kubeClient, owner)
			if err != nil {
				return err
			}
			if !skip {
				strategy, _, _ := unstructured.NestedString(owner.object.Object, "spec", "postBuild", "substituteStrategy")
				for _, res := range resources.Resources() {
					outRes, err := kustomize.SubstituteVariables(ctx, kubeClient, *owner.object, res,
						kustomize.SubstituteWithStrict(true),
						kustomize.SubstituteWithAlways(strategy == "Always"))
					if err != nil {
						return fmt.Errorf("post build failed for '%s/%s': %w", res.GetGvk(), res.GetName(), err)
					}
					if outRes != nil {
						if _, err := resources.Replace(outRes); err != nil {
							return fmt.Errorf("replacing substituted resource %s: %w", res.GetName(), err)
						}
					}
				}
			}
		}
		builtObjects, err := resMapObjects(resources)
		if err != nil {
			return err
		}
		*objects = builtObjects
	case fluxcdv1.ResourceSetKind:
		if err := copyResourceSetData(ctx, kubeClient, *objects, owner); err != nil {
			return err
		}
	case fluxcdv1.FluxHelmReleaseKind:
		if err := applyHelmPostRenderers(objects, owner.object); err != nil {
			return err
		}
	}
	return nil
}

// kustomizeBuildSpec holds the Kustomize transforms shared by the Kustomization
// spec and the HelmRelease post-renderers.
type kustomizeBuildSpec struct {
	TargetNamespace string               `json:"targetNamespace,omitempty"`
	NamePrefix      string               `json:"namePrefix,omitempty"`
	NameSuffix      string               `json:"nameSuffix,omitempty"`
	Patches         []apikustomize.Patch `json:"patches,omitempty"`
	Images          []apikustomize.Image `json:"images,omitempty"`
}

// isEmpty reports whether no transform is configured.
func (s kustomizeBuildSpec) isEmpty() bool {
	return s.TargetNamespace == "" && s.NamePrefix == "" && s.NameSuffix == "" &&
		len(s.Patches) == 0 && len(s.Images) == 0
}

// helmPostRenderer mirrors a HelmRelease spec.postRenderers entry.
type helmPostRenderer struct {
	Kustomize *kustomizeBuildSpec `json:"kustomize,omitempty"`
}

// applyHelmPostRenderers runs the HelmRelease Kustomize post-renderers over
// the objects in order, the same way helm-controller post-renders the chart
// output before Helm adds its release metadata.
func applyHelmPostRenderers(objects *[]*unstructured.Unstructured, helmRelease *unstructured.Unstructured) error {
	raw, found, err := unstructured.NestedFieldNoCopy(helmRelease.Object, "spec", "postRenderers")
	if err != nil || !found {
		return err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("encoding postRenderers: %w", err)
	}
	var renderers []helmPostRenderer
	if err := json.Unmarshal(data, &renderers); err != nil {
		return fmt.Errorf("decoding postRenderers: %w", err)
	}

	current := *objects
	for i, renderer := range renderers {
		if renderer.Kustomize == nil || renderer.Kustomize.isEmpty() {
			continue
		}
		resources, err := kustomizeBuild(current, *renderer.Kustomize)
		if err != nil {
			return fmt.Errorf("postRenderers[%d] failed: %w", i, err)
		}
		if current, err = resMapObjects(resources); err != nil {
			return fmt.Errorf("postRenderers[%d] failed: %w", i, err)
		}
	}
	*objects = current
	return nil
}

// kustomizeBuild applies the transforms to the objects with a Kustomize build
// over an in-memory filesystem, like the helm-controller post-renderer.
func kustomizeBuild(objects []*unstructured.Unstructured, spec kustomizeBuildSpec) (resmap.ResMap, error) {
	const input = "all.yaml"
	fs := filesys.MakeFsInMemory()
	manifest, err := ssautil.ObjectsToYAML(objects)
	if err != nil {
		return nil, fmt.Errorf("encoding build input: %w", err)
	}
	if err := fs.WriteFile(input, []byte(manifest)); err != nil {
		return nil, fmt.Errorf("writing build input: %w", err)
	}

	cfg := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
		Resources:  []string{input},
		Namespace:  spec.TargetNamespace,
		NamePrefix: spec.NamePrefix,
		NameSuffix: spec.NameSuffix,
	}
	for _, patch := range spec.Patches {
		cfg.Patches = append(cfg.Patches, kustypes.Patch{
			Patch:  patch.Patch,
			Target: adaptKustomizeSelector(patch.Target),
		})
	}
	for _, image := range spec.Images {
		cfg.Images = append(cfg.Images, kustypes.Image{
			Name:    image.Name,
			NewName: image.NewName,
			NewTag:  image.NewTag,
			Digest:  image.Digest,
		})
	}
	kustomization, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encoding build definition: %w", err)
	}
	if err := fs.WriteFile("kustomization.yaml", kustomization); err != nil {
		return nil, fmt.Errorf("writing build definition: %w", err)
	}
	return kustomize.Build(fs, ".")
}

// adaptKustomizeSelector converts a Flux patch target to a Kustomize selector.
func adaptKustomizeSelector(selector *apikustomize.Selector) *kustypes.Selector {
	if selector == nil {
		return nil
	}
	output := &kustypes.Selector{}
	output.Gvk.Group = selector.Group
	output.Gvk.Kind = selector.Kind
	output.Gvk.Version = selector.Version
	output.Name = selector.Name
	output.Namespace = selector.Namespace
	output.LabelSelector = selector.LabelSelector
	output.AnnotationSelector = selector.AnnotationSelector
	return output
}

// warnForMissingSubstituteFrom reports non-optional missing references and
// skips all substitution so unresolved variables remain visible to dry-run.
func warnForMissingSubstituteFrom(ctx context.Context, kubeClient ctrlclient.Client, owner *diffOwner) (bool, error) {
	references, found, err := unstructured.NestedSlice(owner.object.Object, "spec", "postBuild", "substituteFrom")
	if err != nil || !found {
		return false, err
	}

	missing := false
	for _, reference := range references {
		ref, ok := reference.(map[string]any)
		if !ok {
			continue
		}
		optional, _, err := unstructured.NestedBool(ref, "optional")
		if err != nil {
			return false, fmt.Errorf("invalid postBuild substituteFrom optional field: %w", err)
		}
		if optional {
			continue
		}
		kind := nestedString(ref, "kind")
		name := nestedString(ref, "name")
		if name == "" || (kind != "ConfigMap" && kind != "Secret") {
			continue
		}
		source := &unstructured.Unstructured{}
		source.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: kind})
		err = kubeClient.Get(ctx, ctrlclient.ObjectKey{Name: name, Namespace: owner.ref.Namespace}, source)
		switch {
		case err == nil:
		case apierrors.IsNotFound(err):
			missing = true
			appendDiffWarning(&owner.warnings, fmt.Sprintf("substituteFrom %s %s/%s not found in the cluster, variables left unresolved",
				kind, owner.ref.Namespace, name))
		default:
			return false, fmt.Errorf("failed to read substituteFrom %s %s/%s: %w", kind, owner.ref.Namespace, name, err)
		}
	}
	return missing, nil
}

// copyResourceSetData resolves copyFrom references while leaving missing
// sources empty for the dry-run and reporting them in the diff header.
func copyResourceSetData(ctx context.Context, kubeClient ctrlclient.Client, objects []*unstructured.Unstructured,
	owner *diffOwner) error {
	for _, object := range objects {
		if err := copier.CopyResources(ctx, kubeClient, []*unstructured.Unstructured{object}); err != nil {
			if apierrors.IsNotFound(err) {
				source := object.GetAnnotations()[fluxcdv1.CopyFromAnnotation]
				appendDiffWarning(&owner.warnings, fmt.Sprintf("copyFrom source %s %s not found in the cluster, data left empty",
					object.GetKind(), source))
				continue
			}
			return err
		}
	}
	return nil
}

// appendDiffWarning adds a warning once while preserving discovery order.
func appendDiffWarning(warnings *[]string, warning string) {
	for _, existing := range *warnings {
		if existing == warning {
			return
		}
	}
	*warnings = append(*warnings, warning)
}

// buildKustomizationResources runs the Kustomization build transforms over the
// manifest when the owner defines any, and reports whether a build happened.
func buildKustomizationResources(objects []*unstructured.Unstructured,
	owner *unstructured.Unstructured) (resmap.ResMap, bool, error) {
	spec, err := decodeKustomizeBuildSpec(owner)
	if err != nil {
		return nil, false, err
	}
	if spec.isEmpty() {
		return nil, false, nil
	}
	resources, err := kustomizeBuild(objects, spec)
	if err != nil {
		return nil, false, fmt.Errorf("kustomize build failed: %w", err)
	}
	return resources, true, nil
}

// requiresKustomizationBuild reports whether the owner has Kustomization build transforms.
func requiresKustomizationBuild(owner *unstructured.Unstructured) bool {
	spec, err := decodeKustomizeBuildSpec(owner)
	return err == nil && !spec.isEmpty()
}

// decodeKustomizeBuildSpec extracts the build transforms from a Kustomization spec.
func decodeKustomizeBuildSpec(owner *unstructured.Unstructured) (kustomizeBuildSpec, error) {
	var spec kustomizeBuildSpec
	raw, found, err := unstructured.NestedFieldNoCopy(owner.Object, "spec")
	if err != nil || !found {
		return spec, err
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return spec, fmt.Errorf("encoding Kustomization spec: %w", err)
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("decoding Kustomization spec: %w", err)
	}
	return spec, nil
}

// hasPostBuild reports whether postBuild is explicitly configured.
func hasPostBuild(owner *unstructured.Unstructured) bool {
	value, found, _ := unstructured.NestedFieldNoCopy(owner.Object, "spec", "postBuild")
	return found && value != nil
}

// objectsResMap converts unstructured objects to a Kustomize resource map in manifest order.
func objectsResMap(objects []*unstructured.Unstructured) (resmap.ResMap, error) {
	manifest, err := ssautil.ObjectsToYAML(objects)
	if err != nil {
		return nil, fmt.Errorf("encoding objects for postBuild substitution: %w", err)
	}
	factory := provider.NewDefaultDepProvider().GetResourceFactory()
	resources, err := resmap.NewFactory(factory).NewResMapFromBytes([]byte(manifest))
	if err != nil {
		return nil, fmt.Errorf("loading objects for postBuild substitution: %w", err)
	}
	return resources, nil
}

// resMapObjects converts a Kustomize resource map to unstructured objects in build order.
func resMapObjects(resources resmap.ResMap) ([]*unstructured.Unstructured, error) {
	manifest, err := resources.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("encoding transformed objects: %w", err)
	}
	objects, err := ssautil.ReadObjects(strings.NewReader(string(manifest)))
	if err != nil {
		return nil, fmt.Errorf("decoding transformed objects: %w", err)
	}
	return objects, nil
}

// applyHelmReleaseMetadata sets Helm ownership metadata and default namespaces.
func applyHelmReleaseMetadata(kubeClient ctrlclient.Client, objects []*unstructured.Unstructured,
	helmRelease *unstructured.Unstructured) error {
	releaseName, releaseNamespace := helmReleaseIdentity(helmRelease)
	crdScopes := manifestCRDScopes(objects)
	for _, object := range objects {
		if object.GetKind() != "CustomResourceDefinition" {
			setHelmStandardMetadata(object, releaseName, releaseNamespace)
		}

		if object.GetNamespace() != "" {
			continue
		}
		namespaced, err := apiutil.IsObjectNamespaced(object, kubeClient.Scheme(), kubeClient.RESTMapper())
		if err != nil {
			if !meta.IsNoMatchError(err) {
				return fmt.Errorf("failed to determine if %s is namespace scoped: %w", object.GetKind(), err)
			}
			var found bool
			namespaced, found = crdScopes[object.GroupVersionKind().Group+"/"+object.GetKind()]
			if !found {
				continue
			}
		}
		if namespaced {
			object.SetNamespace(releaseNamespace)
		}
	}
	return nil
}

// applyHelmMetadataForLiveCRD distinguishes CRDs rendered from templates from
// CRDs shipped in crds/ by checking the Helm release annotation on the live CRD.
func applyHelmMetadataForLiveCRD(desired, live *unstructured.Unstructured, owner *diffOwner) {
	if owner == nil || owner.ref.Kind != fluxcdv1.FluxHelmReleaseKind || desired.GetKind() != "CustomResourceDefinition" {
		return
	}
	if _, found := live.GetAnnotations()["meta.helm.sh/release-name"]; !found {
		return
	}
	releaseName, releaseNamespace := helmReleaseIdentity(owner.object)
	setHelmStandardMetadata(desired, releaseName, releaseNamespace)
}

// setHelmStandardMetadata sets the labels and annotations added by Helm to
// release-managed resources.
func setHelmStandardMetadata(object *unstructured.Unstructured, releaseName, releaseNamespace string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/managed-by"] = "Helm"
	object.SetLabels(labels)

	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["meta.helm.sh/release-name"] = releaseName
	annotations["meta.helm.sh/release-namespace"] = releaseNamespace
	object.SetAnnotations(annotations)
}

// helmReleaseIdentity derives the effective Helm release name and namespace.
func helmReleaseIdentity(helmRelease *unstructured.Unstructured) (string, string) {
	targetNamespace, _, _ := unstructured.NestedString(helmRelease.Object, "spec", "targetNamespace")
	releaseNamespace := targetNamespace
	if releaseNamespace == "" {
		releaseNamespace = helmRelease.GetNamespace()
	}
	releaseName, _, _ := unstructured.NestedString(helmRelease.Object, "spec", "releaseName")
	if releaseName == "" {
		releaseName = helmRelease.GetName()
		if targetNamespace != "" {
			releaseName = targetNamespace + "-" + releaseName
		}
	}
	return shortenHelmReleaseName(releaseName), releaseNamespace
}

// shortenHelmReleaseName mirrors helm-controller's release.ShortenName.
func shortenHelmReleaseName(name string) string {
	if len(name) <= 53 {
		return name
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
	return name[:40] + "-" + sum[:12]
}

// manifestCRDScopes indexes namespaced custom kinds declared by manifest CRDs.
func manifestCRDScopes(objects []*unstructured.Unstructured) map[string]bool {
	scopes := make(map[string]bool)
	for _, object := range objects {
		if object.GetAPIVersion() != "apiextensions.k8s.io/v1" || object.GetKind() != "CustomResourceDefinition" {
			continue
		}
		group, _, _ := unstructured.NestedString(object.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(object.Object, "spec", "names", "kind")
		scope, _, _ := unstructured.NestedString(object.Object, "spec", "scope")
		if group != "" && kind != "" {
			scopes[group+"/"+kind] = scope == "Namespaced"
		}
	}
	return scopes
}

// commonMetadata extracts spec.commonMetadata labels and annotations.
func commonMetadata(object *unstructured.Unstructured) (map[string]string, map[string]string, error) {
	labels, _, err := unstructured.NestedStringMap(object.Object, "spec", "commonMetadata", "labels")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid commonMetadata labels: %w", err)
	}
	annotations, _, err := unstructured.NestedStringMap(object.Object, "spec", "commonMetadata", "annotations")
	if err != nil {
		return nil, nil, fmt.Errorf("invalid commonMetadata annotations: %w", err)
	}
	return labels, annotations, nil
}
