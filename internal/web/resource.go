// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceHandler handles GET /api/v1/resource requests and returns a single Flux resource by kind, name and namespace.
// Query parameters: kind, name, namespace (all required)
// Example: /api/v1/resource?kind=FluxInstance&name=flux&namespace=flux-system
func (r *Router) ResourceHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")

	// Validate required parameters
	if kind == "" || name == "" || namespace == "" {
		http.Error(w, "Missing required parameters: kind, name, namespace", http.StatusBadRequest)
		return
	}

	// Get the resource from the cluster
	resource, err := r.GetResource(req.Context(), kind, name, namespace)
	if err != nil {
		r.log.Error(err, "failed to get resource", "url", req.URL.String(),
			"kind", kind, "name", name, "namespace", namespace)
		http.Error(w, fmt.Sprintf("Failed to get resource: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Encode and send the response
	if err := json.NewEncoder(w).Encode(resource.Object); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetResource fetches a single Flux resource by kind, name and namespace,
// and injects the inventory into the .status.inventory field before returning it.
func (r *Router) GetResource(ctx context.Context, kind, name, namespace string) (*unstructured.Unstructured, error) {
	// Get the preferred GVK for the kind
	gvk, err := r.preferredFluxGVK(kind)
	if err != nil {
		return nil, fmt.Errorf("unable to get GVK for kind %s: %w", kind, err)
	}

	// Create an unstructured object to fetch the resource
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*gvk)

	// Create the object key
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	// Fetch the resource from the cluster
	if err := r.kubeClient.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("unable to get resource %s/%s in namespace %s: %w", kind, name, namespace, err)
	}

	// Get the inventory for this resource
	inventory := r.getInventory(ctx, *obj)

	// Inject/override the .status.inventory field with the extracted inventory
	if len(inventory) > 0 {
		// Convert inventory entries to the format expected in status.inventory.entries
		entries := make([]any, 0, len(inventory))
		for _, entry := range inventory {
			entries = append(entries, map[string]any{
				"name":       entry.Name,
				"namespace":  entry.Namespace,
				"kind":       entry.Kind,
				"apiVersion": entry.APIVersion,
			})
		}

		// Set the inventory in the status field
		if err := unstructured.SetNestedSlice(obj.Object, entries, "status", "inventory"); err != nil {
			return nil, fmt.Errorf("unable to set inventory in status: %w", err)
		}
	}

	// Get the source reference and inject the source details if available
	if source, err := r.getReconcilerSource(ctx, *obj); err == nil && source != nil {
		sourceMap := map[string]any{
			"kind":           source.Kind,
			"name":           source.Name,
			"namespace":      source.Namespace,
			"url":            source.URL,
			"originURL":      source.OriginURL,
			"originRevision": source.OriginRevision,
			"status":         source.Status,
			"message":        source.Message,
		}
		if err := unstructured.SetNestedMap(obj.Object, sourceMap, "status", "sourceRef"); err != nil {
			return nil, fmt.Errorf("unable to set source in spec: %w", err)
		}
	}

	cleanObjectForExport(obj, true)
	return obj, nil
}
