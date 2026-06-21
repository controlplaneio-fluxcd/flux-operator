// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// WorkloadItem represents a single workload request.
type WorkloadItem struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// WorkloadsRequest represents the request body for POST /api/v1/workloads.
type WorkloadsRequest struct {
	Workloads []WorkloadItem `json:"workloads"`
}

// WorkloadsHandler handles POST /api/v1/workloads requests and returns the status
// of the specified workloads.
func (h *Handler) WorkloadsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var wReq WorkloadsRequest
	if err := json.NewDecoder(req.Body).Decode(&wReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Return empty array if no workloads
	if len(wReq.Workloads) == 0 {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"workloads": []WorkloadStatus{}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Fetch status for all workloads
	workloads := h.GetWorkloadsStatus(req.Context(), wReq.Workloads)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"workloads": workloads}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetWorkloadsStatus fetches the status for the specified workloads.
// Workloads are queried in parallel with a concurrency limit of 4.
func (h *Handler) GetWorkloadsStatus(ctx context.Context, workloads []WorkloadItem) []WorkloadStatus {
	result := make([]WorkloadStatus, len(workloads))

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Semaphore to limit concurrent requests to 4
	sem := make(chan struct{}, 4)

	for i, item := range workloads {
		wg.Add(1)
		go func(i int, item WorkloadItem) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			ws, err := h.GetWorkloadStatus(ctx, item.Kind, item.Name, item.Namespace, false)
			if err != nil {
				var statusMessage string
				switch {
				case errors.IsNotFound(err):
					statusMessage = "Workload not found in the cluster"
				case errors.IsForbidden(err):
					statusMessage = "User does not have access to the workload"
				default:
					statusMessage = "Internal error while fetching workload"
					log.FromContext(ctx).Error(err, "failed to get workload status",
						"kind", item.Kind,
						"name", item.Name,
						"namespace", item.Namespace)
				}

				mu.Lock()
				result[i] = WorkloadStatus{
					Kind:          item.Kind,
					Name:          item.Name,
					Namespace:     item.Namespace,
					Status:        "NotFound",
					StatusMessage: statusMessage,
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			result[i] = *ws
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()

	return result
}

// WorkloadsListHandler handles GET /api/v1/workloads requests and returns the
// Flux-managed workloads (Deployment, StatefulSet, DaemonSet, CronJob) from the
// cached, inventory-derived workload index, filtered by the user's namespace
// access. Supports optional query parameters: kind, name, namespace.
// Example: /api/v1/workloads?kind=Deployment&namespace=flux-system
func (h *Handler) WorkloadsListHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")

	// Query the cached workload index with RBAC filtering.
	workloads := h.GetCachedWorkloads(req.Context(), kind, name, namespace, 2500)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"workloads": workloads}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// WorkloadsSearchHandler handles GET /api/v1/workloads/search requests and
// returns Flux-managed workloads from the cached workload index for the global
// quick-search. Results are filtered by name with wildcard support and capped at
// a small limit. Supports optional query parameters: kind, name, namespace.
// Example: /api/v1/workloads/search?name=podinfo
func (h *Handler) WorkloadsSearchHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters.
	queryParams := req.URL.Query()
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	kind := queryParams.Get("kind")

	// If name does not contain a wildcard, wrap it to perform a partial match.
	if name != "" && !hasWildcard(name) {
		name = "*" + name + "*"
	}

	// Query the cached workload index with RBAC filtering.
	workloads := h.GetCachedWorkloads(req.Context(), kind, name, namespace, 10)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"workloads": workloads}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetCachedWorkloads returns workloads from the cached workload index filtered
// by the given criteria and the user's namespace access. If name and namespace
// filters are empty, it returns workloads across all namespaces the user can
// access (subject to RBAC on the parent reconcilers via ListUserNamespaces).
func (h *Handler) GetCachedWorkloads(ctx context.Context, kind, name, namespace string, limit int) []reporter.WorkloadRef {
	// Get user-visible namespaces for RBAC filtering.
	userNamespaces, allNamespaces, err := h.kubeClient.ListUserNamespaces(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to list user namespaces for cached workloads")
		return []reporter.WorkloadRef{}
	}

	// If the user has no access to any namespace, return empty results.
	if !allNamespaces && len(userNamespaces) == 0 {
		return []reporter.WorkloadRef{}
	}

	// For cluster-wide access, pass nil (no RBAC filtering).
	// Otherwise, pass the user's namespace list.
	var allowedNamespaces []string
	if !allNamespaces {
		allowedNamespaces = userNamespaces
	}

	return h.workloadIndex.SearchWorkloads(allowedNamespaces, kind, name, namespace, limit)
}
