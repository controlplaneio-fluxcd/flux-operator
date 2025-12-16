// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
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
func (r *Router) WorkloadsHandler(w http.ResponseWriter, req *http.Request) {
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
	workloads := r.GetWorkloadsStatus(req.Context(), wReq.Workloads)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"workloads": workloads}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetWorkloadsStatus fetches the status for the specified workloads.
// Workloads are queried in parallel with a concurrency limit of 4.
func (r *Router) GetWorkloadsStatus(ctx context.Context, workloads []WorkloadItem) []WorkloadStatus {
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

			ws, err := r.GetWorkloadStatus(ctx, item.Kind, item.Name, item.Namespace)
			if err != nil {
				var statusMessage string
				switch {
				case errors.IsNotFound(err):
					statusMessage = "Workload not found in the cluster"
				case errors.IsForbidden(err):
					statusMessage = "User does not have access to the workload or for listing its pods"
				default:
					statusMessage = "Internal error while fetching workload"
					r.log.Error(err, "failed to get workload status",
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
