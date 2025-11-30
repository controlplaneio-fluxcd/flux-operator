// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FavoriteItem represents a single favorite resource request.
type FavoriteItem struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// FavoritesRequest represents the request body for POST /api/v1/favorites.
type FavoritesRequest struct {
	Favorites []FavoriteItem `json:"favorites"`
}

// FavoritesHandler handles POST /api/v1/favorites requests and returns the status
// of the specified favorite resources.
func (r *Router) FavoritesHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var favReq FavoritesRequest
	if err := json.NewDecoder(req.Body).Decode(&favReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Return empty array if no favorites
	if len(favReq.Favorites) == 0 {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"resources": []ResourceStatus{}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Fetch status for all favorites
	resources, err := r.GetFavoritesStatus(req.Context(), favReq.Favorites)
	if err != nil {
		r.log.Error(err, "failed to get favorites status")
		resources = []ResourceStatus{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"resources": resources}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetFavoritesStatus fetches the status for the specified favorite resources.
// Resources are queried in parallel with a concurrency limit of 4.
func (r *Router) GetFavoritesStatus(ctx context.Context, favorites []FavoriteItem) ([]ResourceStatus, error) {
	var result []ResourceStatus
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(favorites))

	// Semaphore to limit concurrent requests to 4
	sem := make(chan struct{}, 4)

	for _, fav := range favorites {
		wg.Add(1)
		go func(fav FavoriteItem) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			gvk, err := r.preferredFluxGVK(fav.Kind)
			if err != nil {
				if strings.Contains(err.Error(), "no matches for kind") {
					return
				}
				errChan <- err
				return
			}

			obj := unstructured.Unstructured{}
			obj.SetGroupVersionKind(*gvk)

			err = r.kubeClient.Get(ctx, client.ObjectKey{
				Namespace: fav.Namespace,
				Name:      fav.Name,
			}, &obj)

			if err != nil {
				// Resource not found - skip it (deleted favorites are handled by frontend)
				return
			}

			rs := r.resourceStatusFromUnstructured(obj)
			mu.Lock()
			result = append(result, rs)
			mu.Unlock()
		}(fav)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return result, nil
}
