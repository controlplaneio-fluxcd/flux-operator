// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// FluxManagedObject represents a Kubernetes object managed by a Flux reconciler.
// It contains an identifier for the object and information about the Flux reconciler
// that manages it.
type FluxManagedObject struct {
	ID         string
	Reconciler *FluxManagedObjectReconciler
	Source     *FluxManagedObjectSource
}

// FluxManagedObjectReconciler represents a Flux reconciler (such as Kustomization,
// HelmRelease, or ResourceSet) that manages Kubernetes objects. It contains the
// reconciler's identity, status, and operational information.
type FluxManagedObjectReconciler struct {
	Kind           string
	Name           string
	Namespace      string
	Ready          string
	ReadyMessage   string
	LastReconciled string
	Revision       string
}

// FluxManagedObjectSource holds the Flux source info of a managed object.
// It includes the upstream URL, origin URL, and origin revision.
type FluxManagedObjectSource struct {
	Kind           string
	Name           string
	Namespace      string
	URL            string
	OriginURL      string
	OriginRevision string
}

// NewFluxManagedObject creates a new FluxManagedObject from a Kubernetes object.
// It generates an ID based on the object's kind, namespace, and name.
func NewFluxManagedObject(obj *unstructured.Unstructured) *FluxManagedObject {
	id := fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())
	if obj.GetNamespace() != "" {
		id = fmt.Sprintf("%s/%s/%s", obj.GetKind(), obj.GetNamespace(), obj.GetName())
	}

	return &FluxManagedObject{
		ID: id,
	}
}

// Compute queries the cluster for the reconciler and the source of the Flux managed object.
func (f *FluxManagedObject) Compute(
	ctx context.Context,
	kubeClient client.Client,
	reconciler *FluxManagedObjectReconciler,
) error {
	gvk, err := preferredFluxGVK(reconciler.Kind, kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("unable to get gvk for kind %s: %w", reconciler.Kind, err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*gvk)

	namespacedName := types.NamespacedName{
		Namespace: reconciler.Namespace,
		Name:      reconciler.Name,
	}

	err = kubeClient.Get(ctx, namespacedName, obj)
	if err != nil {
		return fmt.Errorf("unable to get %s %s: %w", reconciler.Kind, namespacedName, err)
	}

	// Extract reconciler status information
	extractReconcilerStatus(obj, reconciler)

	// Extract source information
	source, err := f.extractSource(ctx, kubeClient, obj, reconciler)
	if err == nil {
		f.Source = source
	}

	f.Reconciler = reconciler
	return nil
}

// extractReconcilerStatus extracts status information from a reconciler object.
func extractReconcilerStatus(obj *unstructured.Unstructured, reconciler *FluxManagedObjectReconciler) {
	// Initialize defaults
	reconciler.Ready = "Unknown"
	reconciler.ReadyMessage = "Not initialized"
	reconciler.LastReconciled = "Unknown"
	reconciler.Revision = "Unknown"

	// Extract the ready status from conditions
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
		for _, cond := range conditions {
			if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
				reconciler.Ready = condition["status"].(string)
				if msg, exists := condition["message"]; exists {
					reconciler.ReadyMessage = msg.(string)
				}
				if lastTransitionTime, exists := condition["lastTransitionTime"]; exists {
					reconciler.LastReconciled = lastTransitionTime.(string)
				}
			}
		}
	}

	// Check for suspend annotation
	if ssautil.AnyInMetadata(obj,
		map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
		reconciler.Ready = "Suspended"
	}

	// Check for suspend spec field
	if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
		reconciler.Ready = "Suspended"
	}

	// Extract the revision from status
	if lastAttemptedRevision, found, err := unstructured.NestedString(obj.Object, "status", "lastAttemptedRevision"); found && err == nil {
		reconciler.Revision = lastAttemptedRevision
	}
	if lastAppliedRevision, found, err := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision"); found && err == nil {
		reconciler.Revision = lastAppliedRevision
	}
}

// extractSource extracts source information from a reconciler object.
func (f *FluxManagedObject) extractSource(
	ctx context.Context,
	kubeClient client.Client,
	obj *unstructured.Unstructured,
	reconciler *FluxManagedObjectReconciler,
) (*FluxManagedObjectSource, error) {
	switch reconciler.Kind {
	case fluxcdv1.FluxKustomizationKind:
		if sourceRef, found, err := unstructured.NestedMap(obj.Object, "spec", "sourceRef"); found && err == nil {
			if name, exists := sourceRef["name"]; exists {
				if kind, exists := sourceRef["kind"]; exists {
					namespace := reconciler.Namespace // Default to reconciler's namespace
					if ns, exists := sourceRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return f.buildSourceFromRef(ctx, kubeClient, kind.(string), namespace, name.(string))
				}
			}
		}
	case fluxcdv1.FluxHelmReleaseKind:
		// Try spec.chartRef (direct chart reference)
		if chartRef, found, err := unstructured.NestedMap(obj.Object, "spec", "chartRef"); found && err == nil {
			if name, exists := chartRef["name"]; exists {
				if kind, exists := chartRef["kind"]; exists {
					namespace := reconciler.Namespace // Default to reconciler's namespace
					if ns, exists := chartRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return f.buildSourceFromRef(ctx, kubeClient, kind.(string), namespace, name.(string))
				}
			}
		} else if chartSourceRef, found, err := unstructured.NestedMap(obj.Object, "spec", "chart", "spec", "sourceRef"); found && err == nil {
			// Fall back to spec.chart.spec.sourceRef (chart from Git/Helm repository)
			if name, exists := chartSourceRef["name"]; exists {
				if kind, exists := chartSourceRef["kind"]; exists {
					namespace := reconciler.Namespace // Default to reconciler's namespace
					if ns, exists := chartSourceRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return f.buildSourceFromRef(ctx, kubeClient, kind.(string), namespace, name.(string))
				}
			}
		}
	}
	return nil, fmt.Errorf("no source reference found")
}

// buildSourceFromRef builds a FluxManagedObjectSource from source reference information.
func (f *FluxManagedObject) buildSourceFromRef(
	ctx context.Context,
	kubeClient client.Client,
	kind, namespace, name string,
) (*FluxManagedObjectSource, error) {
	s, err := extractSourceURL(ctx, kubeClient, kind, namespace, name)
	if err != nil {
		return nil, err
	}

	return &FluxManagedObjectSource{
		Kind:           kind,
		Name:           name,
		Namespace:      namespace,
		URL:            s.URL,
		OriginURL:      s.OriginURL,
		OriginRevision: s.OriginRevision,
	}, nil
}

// extractSourceURL fetches a source object from the cluster and extracts its URL and origin.
// The origin info is only available for OCI Artifacts.
func extractSourceURL(
	ctx context.Context,
	kubeClient client.Client,
	kind, namespace, name string,
) (*FluxManagedObjectSource, error) {
	gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
	if err != nil {
		return nil, fmt.Errorf("unable to get gvk for kind %s: %w", kind, err)
	}

	sourceObj := &unstructured.Unstructured{}
	sourceObj.SetGroupVersionKind(*gvk)

	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	err = kubeClient.Get(ctx, namespacedName, sourceObj)
	if err != nil {
		return nil, fmt.Errorf("unable to get %s %s: %w", kind, namespacedName, err)
	}

	var url, originURL, originRevision string

	switch kind {
	case fluxcdv1.FluxHelmChartKind:
		// For HelmChart, the URL is in its sourceRef object
		if sourceRef, found, err := unstructured.NestedMap(sourceObj.Object, "spec", "sourceRef"); found && err == nil {
			if chartSourceName, exists := sourceRef["name"]; exists {
				if chartSourceKind, exists := sourceRef["kind"]; exists {
					chartSourceNamespace := sourceObj.GetNamespace()
					if ns, exists := sourceRef["namespace"]; exists && ns != "" {
						chartSourceNamespace = ns.(string)
					}
					return extractSourceURL(
						ctx,
						kubeClient, chartSourceKind.(string),
						chartSourceNamespace,
						chartSourceName.(string),
					)
				}
			}
		}
	case fluxcdv1.FluxBucketKind:
		// For Bucket, the URL is in spec.endpoint
		if endpoint, found, err := unstructured.NestedString(sourceObj.Object, "spec", "endpoint"); found && err == nil {
			url = endpoint
		}
	case fluxcdv1.FluxExternalArtifactKind:
		if u, found, err := unstructured.NestedString(sourceObj.Object, "status", "artifact", "url"); found && err == nil {
			url = u
		}
	default:
		// For all other types, the URL is in spec.url
		if sourceURL, found, err := unstructured.NestedString(sourceObj.Object, "spec", "url"); found && err == nil {
			url = sourceURL
		}
	}

	// Extract origin from status.artifact.metadata['org.opencontainers.image.source']
	// and optionally include 'org.opencontainers.image.revision' if available
	if annotations, found, err := unstructured.NestedStringMap(sourceObj.Object, "status", "artifact", "metadata"); found && err == nil {
		if sourceOrigin, exists := annotations["org.opencontainers.image.source"]; exists {
			originURL = sourceOrigin
		}
		if revision, exists := annotations["org.opencontainers.image.revision"]; exists {
			originRevision = revision
		}
	}

	if url == "" {
		return nil, fmt.Errorf("no URL found in %s/%s/%s", kind, namespace, name)
	}

	return &FluxManagedObjectSource{
		URL:            url,
		OriginURL:      originURL,
		OriginRevision: originRevision,
	}, nil
}
