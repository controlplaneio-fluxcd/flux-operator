// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
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
	resources := r.GetFavoritesStatus(req.Context(), favReq.Favorites)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"resources": resources}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetFavoritesStatus fetches the status for the specified favorite resources.
// Resources are queried in parallel with a concurrency limit of 4.
func (r *Router) GetFavoritesStatus(ctx context.Context, favorites []FavoriteItem) []ResourceStatus {
	result := make([]ResourceStatus, len(favorites))

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Semaphore to limit concurrent requests to 4
	sem := make(chan struct{}, 4)

	for i, fav := range favorites {
		wg.Add(1)
		go func(i int, fav FavoriteItem) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			storeNotFound := func(message string) {
				mu.Lock()
				result[i] = ResourceStatus{
					Kind:      fav.Kind,
					Name:      fav.Name,
					Namespace: fav.Namespace,
					Status:    "NotFound",
					Message:   message,
				}
				mu.Unlock()
			}

			gvk, err := r.preferredFluxGVK(ctx, fav.Kind)
			if err != nil {
				var message string
				switch {
				case strings.Contains(err.Error(), "no matches for kind"):
					message = "Resource kind not found in the cluster"
				default:
					message = "Internal error while fetching resource kind"
					r.log.Error(err, "failed to get favorite resource kind",
						"kind", fav.Kind,
						"name", fav.Name,
						"namespace", fav.Namespace)
				}

				storeNotFound(message)
				return
			}

			obj := unstructured.Unstructured{}
			obj.SetGroupVersionKind(*gvk)

			err = r.kubeClient.GetClient(ctx).Get(ctx, client.ObjectKey{
				Namespace: fav.Namespace,
				Name:      fav.Name,
			}, &obj)

			if err != nil {
				var message string
				switch {
				case errors.IsNotFound(err):
					message = "Resource not found in the cluster"
				case errors.IsForbidden(err):
					message = "User does not have access to the resource"
				default:
					message = "Internal error while fetching resource"
					r.log.Error(err, "failed to get favorite resource",
						"kind", fav.Kind,
						"name", fav.Name,
						"namespace", fav.Namespace)
				}

				storeNotFound(message)
				return
			}

			rs := r.resourceStatusFromUnstructured(obj)
			mu.Lock()
			result[i] = rs
			mu.Unlock()
		}(i, fav)
	}

	wg.Wait()

	return result
}
