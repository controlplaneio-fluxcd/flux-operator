// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// traceMaxOwnerDepth bounds the ownerReferences walk from the traced object.
	traceMaxOwnerDepth = 8
	// traceMaxChartDepth bounds the HelmChart to repository indirection walk.
	traceMaxChartDepth = 3
)

// TraceOptions identifies the Kubernetes object to trace
// through the GitOps delivery pipeline.
type TraceOptions struct {
	// APIVersion of the object.
	APIVersion string
	// Kind of the object.
	Kind string
	// Name of the object.
	Name string
	// Namespace of the object, empty for cluster-scoped objects.
	Namespace string
}

// TraceLink describes one object related to the traced Kubernetes resource.
// The fields are declared in the order they are rendered in YAML.
type TraceLink struct {
	// Object identifies the object as Kind/namespace/name,
	// or Kind/name for cluster-scoped objects.
	Object string `json:"object" yaml:"object"`
	// Status is the Ready condition reason for Flux objects,
	// or the kstatus computed status for other kinds.
	Status string `json:"status" yaml:"status"`
	// Suspended is true when the reconciliation of a Flux object is paused.
	Suspended bool `json:"suspended,omitempty" yaml:"suspended,omitempty"`
	// Message is set only when the object is not ready.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	// LastAppliedRevision is the revision applied by a Flux applier.
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty" yaml:"lastAppliedRevision,omitempty"`
	// URL is the address a Flux source pulls from.
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
	// Revision is the revision of the artifact fetched by a Flux source.
	Revision string `json:"revision,omitempty" yaml:"revision,omitempty"`
}

// TraceManager is one object in the upward management path.
type TraceManager struct {
	TraceLink `json:",inline" yaml:",inline"`
	// ManagedBy is the next Flux object up the path, omitted at the top.
	ManagedBy *TraceManager `json:"managedBy,omitempty" yaml:"managedBy,omitempty"`
}

// TraceProducer is the producer of an ExternalArtifact.
type TraceProducer struct {
	TraceLink `json:",inline" yaml:",inline"`
	// BuiltFrom contains the upstream Flux sources combined by the producer.
	BuiltFrom []TraceLink `json:"builtFrom,omitempty" yaml:"builtFrom,omitempty"`
}

// TraceSource describes the source resolved for the nearest Flux applier.
type TraceSource struct {
	// ResolvedFor identifies the applier the source was resolved for.
	ResolvedFor string `json:"resolvedFor" yaml:"resolvedFor"`
	TraceLink   `json:",inline" yaml:",inline"`
	// ProducedBy is the producer of an ExternalArtifact.
	ProducedBy *TraceProducer `json:"producedBy,omitempty" yaml:"producedBy,omitempty"`
	// BuiltFrom contains the upstream Flux sources combined by the resolved source.
	BuiltFrom []TraceLink `json:"builtFrom,omitempty" yaml:"builtFrom,omitempty"`
}

// TraceResult describes the upward management path from a Kubernetes object
// and the source of its delivery pipeline.
type TraceResult struct {
	// Object identifies the traced object.
	Object string `json:"object" yaml:"object"`
	// Unmanaged is true when no Flux ownership labels were found
	// on the object or its owners.
	Unmanaged bool `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
	// ManagedBy is the nearest manager, with each subsequent manager nested recursively.
	ManagedBy *TraceManager `json:"managedBy,omitempty" yaml:"managedBy,omitempty"`
	// Source is the source resolved for the applier nearest to the traced object,
	// omitted when no applier in the path has a source.
	Source *TraceSource `json:"source,omitempty" yaml:"source,omitempty"`
}

// Trace walks from a Kubernetes object up its GitOps delivery pipeline:
// through the ownerReferences to the object carrying Flux ownership labels,
// then through the labels to the chain of managing Flux objects, and
// resolves the source pulled by the applier nearest to the traced object.
func (k *Client) Trace(ctx context.Context, opts TraceOptions) (*TraceResult, error) {
	gvk, err := k.ParseGroupVersionKind(opts.APIVersion, opts.Kind)
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	objKey := types.NamespacedName{Namespace: opts.Namespace, Name: opts.Name}
	if err := k.Client.Get(ctx, objKey, obj); err != nil {
		return nil, fmt.Errorf("unable to get %s %s: %w", opts.Kind, objKey, err)
	}

	result := &TraceResult{Object: objectID(obj)}
	visited := map[string]bool{result.Object: true}
	// candidates holds the traced object and its managers, nearest first,
	// for source resolution.
	candidates := []*unstructured.Unstructured{obj}
	managerSlot := &result.ManagedBy

	labeled, owner, err := k.findFluxOwner(ctx, obj, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to trace %s: %w", result.Object, err)
	}
	if owner == nil {
		// The source of an unmanaged Flux applier is still resolved below.
		result.Unmanaged = true
	} else if id := objectID(labeled); !visited[id] {
		visited[id] = true
		manager := &TraceManager{TraceLink: newTraceLink(labeled)}
		*managerSlot = manager
		managerSlot = &manager.ManagedBy
		candidates = append(candidates, labeled)
	}

	for owner != nil {
		ownerObj, err := k.getFluxObject(ctx, "", owner)
		if err != nil {
			return nil, fmt.Errorf("unable to get %s/%s/%s managing %s: %w",
				owner.Kind, owner.Namespace, owner.Name, result.Object, err)
		}
		id := objectID(ownerObj)
		if visited[id] {
			break
		}
		visited[id] = true
		manager := &TraceManager{TraceLink: newTraceLink(ownerObj)}
		*managerSlot = manager
		managerSlot = &manager.ManagedBy
		candidates = append(candidates, ownerObj)
		owner, _ = fluxOwnerFromLabels(ownerObj.GetLabels())
	}

	// Resolve the source of the nearest applier, as the walk may top out at
	// a ResourceSet or FluxInstance which have no source of their own.
	for _, candidate := range candidates {
		srcRef, srcAPIVersion := sourceRefOf(candidate)
		if srcRef == nil {
			continue
		}
		srcObj, err := k.getSourceObject(ctx, srcAPIVersion, srcRef)
		if err != nil || visited[objectID(srcObj)] {
			break
		}
		result.Source = k.newTraceSource(ctx, objectID(candidate), srcObj, visited)
		break
	}

	return result, nil
}

// newTraceSource builds the relationships of a resolved Flux source, following
// an ExternalArtifact to its producer and listing the spec.sources upstreams.
func (k *Client) newTraceSource(ctx context.Context, resolvedFor string, srcObj *unstructured.Unstructured, visited map[string]bool) *TraceSource {
	visited[objectID(srcObj)] = true
	link := newTraceLink(srcObj)
	link.URL = sourceURL(srcObj)
	source := &TraceSource{
		ResolvedFor: resolvedFor,
		TraceLink:   link,
	}

	// An ExternalArtifact names its producer, e.g. an ArtifactGenerator, in spec.sourceRef.
	if srcObj.GetKind() == fluxcdv1.FluxExternalArtifactKind {
		genRef, genAPIVersion := namedRefAt(srcObj, "spec", "sourceRef")
		if genRef == nil {
			return source
		}
		genObj, err := k.getFluxObject(ctx, genAPIVersion, genRef)
		if err != nil || visited[objectID(genObj)] {
			return source
		}
		visited[objectID(genObj)] = true
		source.ProducedBy = &TraceProducer{
			TraceLink: newTraceLink(genObj),
			BuiltFrom: k.traceBuiltFrom(ctx, genObj, visited),
		}
		return source
	}

	source.BuiltFrom = k.traceBuiltFrom(ctx, srcObj, visited)
	return source
}

// traceBuiltFrom resolves the spec.sources upstreams of a generated source,
// omitting objects already present in the trace.
func (k *Client) traceBuiltFrom(ctx context.Context, producer *unstructured.Unstructured, visited map[string]bool) []TraceLink {
	var builtFrom []TraceLink
	sources, found, err := unstructured.NestedSlice(producer.Object, "spec", "sources")
	if !found || err != nil {
		return builtFrom
	}
	for _, item := range sources {
		src, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := src["kind"].(string)
		name, _ := src["name"].(string)
		if kind == "" || name == "" {
			continue
		}
		namespace, _ := src["namespace"].(string)
		if namespace == "" {
			namespace = producer.GetNamespace()
		}
		upstream, err := k.getSourceObject(ctx, "", &DiffOwnerRef{Kind: kind, Name: name, Namespace: namespace})
		if err != nil || visited[objectID(upstream)] {
			continue
		}
		visited[objectID(upstream)] = true
		upstreamLink := newTraceLink(upstream)
		upstreamLink.URL = sourceURL(upstream)
		builtFrom = append(builtFrom, upstreamLink)
	}
	return builtFrom
}

// findFluxOwner returns the object carrying Flux ownership labels, found on
// the object itself or up its ownerReferences, together with the reference to
// the Flux object managing it. A nil reference means unmanaged.
func (k *Client) findFluxOwner(ctx context.Context, obj *unstructured.Unstructured, depth int) (*unstructured.Unstructured, *DiffOwnerRef, error) {
	if ref, found := fluxOwnerFromLabels(obj.GetLabels()); found {
		return obj, ref, nil
	}
	if depth >= traceMaxOwnerDepth {
		return nil, nil, nil
	}

	for _, ownerRef := range obj.GetOwnerReferences() {
		gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return nil, nil, err
		}

		ownerObj := &unstructured.Unstructured{}
		ownerObj.SetGroupVersionKind(gv.WithKind(ownerRef.Kind))
		ownerKey := types.NamespacedName{Namespace: obj.GetNamespace(), Name: ownerRef.Name}
		if err := k.Client.Get(ctx, ownerKey, ownerObj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, nil, err
		}

		labeled, ref, err := k.findFluxOwner(ctx, ownerObj, depth+1)
		if err != nil || ref != nil {
			return labeled, ref, err
		}
	}
	return nil, nil, nil
}

// getFluxObject fetches a referenced object, resolving its version from the
// API server discovery when the reference carries no apiVersion.
func (k *Client) getFluxObject(ctx context.Context, apiVersion string, ref *DiffOwnerRef) (*unstructured.Unstructured, error) {
	gvk, err := k.ResolveGroupVersionKind(apiVersion, ref.Kind)
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := k.Client.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// getSourceObject fetches a source object, following a HelmChart to the
// repository it references.
func (k *Client) getSourceObject(ctx context.Context, apiVersion string, ref *DiffOwnerRef) (*unstructured.Unstructured, error) {
	obj, err := k.getFluxObject(ctx, apiVersion, ref)
	if err != nil {
		return nil, err
	}
	for depth := 0; depth < traceMaxChartDepth && obj.GetKind() == fluxcdv1.FluxHelmChartKind; depth++ {
		chartSrcRef, chartSrcAPIVersion := namedRefAt(obj, "spec", "sourceRef")
		if chartSrcRef == nil {
			break
		}
		next, err := k.getFluxObject(ctx, chartSrcAPIVersion, chartSrcRef)
		if err != nil {
			break
		}
		obj = next
	}
	return obj, nil
}

// sourceRefOf returns the source reference a Flux object pulls from,
// or nil for kinds without a source.
func sourceRefOf(obj *unstructured.Unstructured) (*DiffOwnerRef, string) {
	var paths [][]string
	switch obj.GetKind() {
	case fluxcdv1.FluxKustomizationKind, fluxcdv1.FluxHelmChartKind, fluxcdv1.FluxExternalArtifactKind:
		paths = [][]string{{"spec", "sourceRef"}}
	case fluxcdv1.FluxHelmReleaseKind:
		paths = [][]string{{"spec", "chartRef"}, {"spec", "chart", "spec", "sourceRef"}}
	default:
		return nil, ""
	}

	for _, path := range paths {
		if ref, apiVersion := namedRefAt(obj, path...); ref != nil {
			return ref, apiVersion
		}
	}
	return nil, ""
}

// namedRefAt reads an object reference at the given field path, defaulting the
// namespace to the parent's. The second return value is the reference's
// apiVersion, empty when the field does not carry one.
func namedRefAt(obj *unstructured.Unstructured, path ...string) (*DiffOwnerRef, string) {
	ref, found, err := unstructured.NestedStringMap(obj.Object, path...)
	if !found || err != nil || ref["kind"] == "" || ref["name"] == "" {
		return nil, ""
	}

	namespace := ref["namespace"]
	if namespace == "" {
		namespace = obj.GetNamespace()
	}
	return &DiffOwnerRef{Kind: ref["kind"], Name: ref["name"], Namespace: namespace}, ref["apiVersion"]
}

// sourceURL extracts the address a Flux source object points at.
func sourceURL(obj *unstructured.Unstructured) string {
	switch obj.GetKind() {
	case fluxcdv1.FluxBucketKind:
		url, _, _ := unstructured.NestedString(obj.Object, "spec", "endpoint")
		return url
	case fluxcdv1.FluxExternalArtifactKind:
		url, _, _ := unstructured.NestedString(obj.Object, "status", "artifact", "url")
		return url
	default:
		url, _, _ := unstructured.NestedString(obj.Object, "spec", "url")
		return url
	}
}

// newTraceLink builds a relationship entry from a live object: Flux objects
// report their Ready condition reason, other kinds the kstatus computed status.
func newTraceLink(obj *unstructured.Unstructured) TraceLink {
	link := TraceLink{Object: objectID(obj)}

	if !fluxcdv1.IsFluxAPI(obj.GetAPIVersion()) {
		res, err := status.Compute(obj)
		if err != nil {
			link.Status = string(status.UnknownStatus)
			link.Message = err.Error()
			return link
		}
		link.Status = string(res.Status)
		if res.Status != status.CurrentStatus {
			link.Message = res.Message
		}
		return link
	}

	link.Status = string(status.UnknownStatus)
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok || cond["type"] != meta.ReadyCondition {
				continue
			}
			if reason, _ := cond["reason"].(string); reason != "" {
				link.Status = reason
			}
			if condStatus, _ := cond["status"].(string); condStatus != "True" {
				link.Message, _ = cond["message"].(string)
			}
			break
		}
	}

	if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); found && err == nil && suspend {
		link.Suspended = true
	}
	if ssautil.AnyInMetadata(obj, map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
		link.Suspended = true
	}

	link.LastAppliedRevision, _, _ = unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
	link.Revision, _, _ = unstructured.NestedString(obj.Object, "status", "artifact", "revision")
	return link
}

// objectID renders an object as Kind/namespace/name,
// or Kind/name for cluster-scoped objects.
func objectID(obj *unstructured.Unstructured) string {
	if obj.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
	}
	return fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
}
