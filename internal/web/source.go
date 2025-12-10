// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ReconcilerSource holds the Flux source info of a managed object.
// It includes the upstream URL, origin URL, and origin revision.
type ReconcilerSource struct {
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	URL            string `json:"url"`
	OriginURL      string `json:"originURL"`
	OriginRevision string `json:"originRevision"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

// getReconcilerSource extracts the source reference from the given Flux reconciler object
// and retrieves the corresponding source URL, origin URL, and origin revision.
func (r *Router) getReconcilerSource(ctx context.Context, obj unstructured.Unstructured) (*ReconcilerSource, error) {
	switch obj.GetKind() {
	case fluxcdv1.FluxKustomizationKind, fluxcdv1.FluxHelmChartKind, fluxcdv1.FluxImageUpdateAutomationKind:
		if sourceRef, found, err := unstructured.NestedMap(obj.Object, "spec", "sourceRef"); found && err == nil {
			if name, exists := sourceRef["name"]; exists {
				if kind, exists := sourceRef["kind"]; exists {
					namespace := obj.GetNamespace() // Default to reconciler's namespace
					if ns, exists := sourceRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return r.extractSourceRef(ctx, kind.(string), namespace, name.(string))
				}
			}
		}
	case fluxcdv1.FluxHelmReleaseKind:
		// Try spec.chartRef (direct chart reference)
		if chartRef, found, err := unstructured.NestedMap(obj.Object, "spec", "chartRef"); found && err == nil {
			if name, exists := chartRef["name"]; exists {
				if kind, exists := chartRef["kind"]; exists {
					namespace := obj.GetNamespace() // Default to reconciler's namespace
					if ns, exists := chartRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return r.extractSourceRef(ctx, kind.(string), namespace, name.(string))
				}
			}
		} else if chartSourceRef, found, err := unstructured.NestedMap(obj.Object, "spec", "chart", "spec", "sourceRef"); found && err == nil {
			// Fall back to spec.chart.spec.sourceRef (chart from Git/Helm repository)
			if name, exists := chartSourceRef["name"]; exists {
				if kind, exists := chartSourceRef["kind"]; exists {
					namespace := obj.GetNamespace() // Default to reconciler's namespace
					if ns, exists := chartSourceRef["namespace"]; exists && ns != "" {
						namespace = ns.(string)
					}
					return r.extractSourceRef(ctx, kind.(string), namespace, name.(string))
				}
			}
		}
	}
	return nil, fmt.Errorf("no source reference found")
}

// extractSourceRef retrieves the source URL, origin URL, and origin revision for a given source reference.
func (r *Router) extractSourceRef(ctx context.Context, kind, namespace, name string) (*ReconcilerSource, error) {
	gvk, err := r.preferredFluxGVK(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("unable to get gvk for kind %s: %w", kind, err)
	}

	sourceObj := &unstructured.Unstructured{}
	sourceObj.SetGroupVersionKind(*gvk)

	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	err = r.kubeClient.GetClient(ctx).Get(ctx, namespacedName, sourceObj)
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
					return r.extractSourceRef(
						ctx,
						chartSourceKind.(string),
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

	status := r.resourceStatusFromUnstructured(*sourceObj)
	return &ReconcilerSource{
		Kind:           kind,
		Name:           name,
		Namespace:      namespace,
		URL:            url,
		OriginURL:      originURL,
		OriginRevision: originRevision,
		Status:         status.Status,
		Message:        status.Message,
	}, nil
}
