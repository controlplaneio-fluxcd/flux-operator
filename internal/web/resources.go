// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// ResourcesHandler handles GET /api/v1/resources requests and returns the status of Flux resources.
// Supports optional query parameters: kind, name, namespace, status
// Example: /api/v1/resources?kind=FluxInstance&name=flux&namespace=flux-system&status=Ready
func (h *Handler) ResourcesHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	status := queryParams.Get("status")

	// Get resource status from the cluster using the request context
	resources, err := h.GetResourcesStatus(req.Context(), kind, name, namespace, status, 2500)
	if err != nil {
		log.FromContext(req.Context()).Error(err, "failed to get resources status")
		// Return empty array instead of error for better UX
		resources = []reporter.ResourceStatus{}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Encode and send the response
	response := map[string]any{"resources": resources}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetResourcesStatusOption defines a functional option for GetResourcesStatus.
type GetResourcesStatusOption func(*getResourcesStatusOptions)

// WithSourcesIfNamespace is a functional option to include source kinds when a specific namespace is provided.
func WithSourcesIfNamespace() GetResourcesStatusOption {
	return func(opts *getResourcesStatusOptions) {
		opts.sourcesIfNamespace = true
	}
}

type getResourcesStatusOptions struct {
	sourcesIfNamespace bool
}

// GetResourcesStatus returns the status for the specified resource kinds and optional name in the given namespace.
// If name is empty, returns the status for all resources of the specified kinds are returned.
// Filters by status (Ready, Failed, Progressing, Suspended, Unknown) if provided.
func (h *Handler) GetResourcesStatus(ctx context.Context, kind, name, namespace, status string,
	matchLimit int, opts ...GetResourcesStatusOption) ([]reporter.ResourceStatus, error) {

	var o getResourcesStatusOptions
	for _, opt := range opts {
		opt(&o)
	}

	// Build kinds array based on query parameter
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Default kinds
		kinds = []string{
			// Appliers
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
		}

		// If option is not set or namespace is specified, add source kinds as well
		if !o.sourcesIfNamespace || namespace != "" {
			kinds = append(kinds,
				fluxcdv1.FluxGitRepositoryKind,
				fluxcdv1.FluxOCIRepositoryKind,
				fluxcdv1.FluxHelmChartKind,
				fluxcdv1.FluxHelmRepositoryKind,
				fluxcdv1.FluxBucketKind,
				fluxcdv1.FluxArtifactGeneratorKind,
				fluxcdv1.ResourceSetInputProviderKind,
			)
		}
	}

	// Prepare list of namespaces to search in
	var namespaces []string
	if namespace != "" {
		namespaces = []string{namespace}
	} else {
		// Check if the user has access to all namespaces
		userNamespaces, all, err := h.kubeClient.ListUserNamespaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list user namespaces: %w", err)
		}

		// If the user has no access to any namespaces, return empty result
		if len(userNamespaces) == 0 {
			return []reporter.ResourceStatus{}, nil
		}

		// If the user has cluster-wide access, we can add FluxInstance to kinds
		if all && kind == "" {
			kinds = append(kinds, fluxcdv1.FluxInstanceKind)
		}

		// If the user does not have access to all namespaces, limit search to their namespaces
		if !all {
			namespaces = userNamespaces
		}
	}

	var result []reporter.ResourceStatus

	if len(kinds) == 0 {
		return nil, errors.New("no resource kinds specified")
	}

	// Set query limit based on number of kinds
	queryLimit := 5000
	if len(kinds) > 1 {
		queryLimit = 2500
	}

	// Query resources for each kind in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, kind := range kinds {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()

			gvk, err := h.preferredFluxGVK(ctx, kind)
			if err != nil {
				if strings.Contains(err.Error(), "no matches for kind") {
					return
				}
				log.FromContext(ctx).Error(err, "failed to get gvk for kind", "kind", kind)
				return
			}

			// Determine which namespaces to query.
			// If namespaces is empty, query all namespaces (cluster-wide access).
			// Otherwise, query each namespace in the list.
			namespacesToQuery := namespaces
			if len(namespacesToQuery) == 0 {
				namespacesToQuery = []string{""}
			}

			var byKindResult []reporter.ResourceStatus
			for _, ns := range namespacesToQuery {
				list := unstructured.UnstructuredList{
					Object: map[string]any{
						"apiVersion": gvk.Group + "/" + gvk.Version,
						"kind":       gvk.Kind,
					},
				}

				listOpts := []client.ListOption{
					client.Limit(queryLimit),
				}
				if ns != "" {
					listOpts = append(listOpts, client.InNamespace(ns))
				}

				// Add name filter if provided and doesn't contain wildcards
				if name != "" && !hasWildcard(name) {
					listOpts = append(listOpts, client.MatchingFields{"metadata.name": name})
				}

				if err := h.kubeClient.GetClient(ctx).List(ctx, &list, listOpts...); err != nil {
					if !apierrors.IsForbidden(err) {
						log.FromContext(ctx).Error(err, "failed to list resources",
							"kind", kind,
							"namespace", ns)
					}
					return
				}

				for _, obj := range list.Items {
					// Filter by name using wildcard matching if needed
					if hasWildcard(name) {
						objName, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")
						if !matchesWildcard(objName, name) {
							continue
						}
					}

					rs := reporter.NewResourceStatus(obj)
					byKindResult = append(byKindResult, rs)
				}
			}

			mu.Lock()
			// cap the results to the specified limit
			if matchLimit > 0 && len(byKindResult) > matchLimit {
				byKindResult = byKindResult[:matchLimit]
			}

			// append to the main result
			result = append(result, byKindResult...)

			mu.Unlock()
		}(kind)
	}

	wg.Wait()

	// Filter by status if specified
	if status != "" {
		filteredResult := make([]reporter.ResourceStatus, 0, len(result))
		for _, rs := range result {
			if rs.Status == status {
				filteredResult = append(filteredResult, rs)
			}
		}
		result = filteredResult
	}

	// Sort resources by LastReconciled timestamp (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastReconciled.Time.After(result[j].LastReconciled.Time)
	})

	return result, nil
}
