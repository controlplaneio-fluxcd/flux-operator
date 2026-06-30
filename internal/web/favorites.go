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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// favoriteWorkloadKinds is the set of Kubernetes workload kinds supported as
// favorites. Unlike supportedWorkloadKinds, it does not include Pod, since pods
// are not standalone favorites.
var favoriteWorkloadKinds = map[string]struct{}{
	workloadKindDeployment:  {},
	workloadKindStatefulSet: {},
	workloadKindDaemonSet:   {},
	workloadKindCronJob:     {},
}

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
func (h *Handler) FavoritesHandler(w http.ResponseWriter, req *http.Request) {
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
		if err := json.NewEncoder(w).Encode(map[string]any{"resources": []reporter.ResourceStatus{}}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Fetch status for all favorites
	resources := h.GetFavoritesStatus(req.Context(), favReq.Favorites)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"resources": resources}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GetFavoritesStatus fetches the status for the specified favorite resources.
// Resources are queried in parallel with a concurrency limit of 4.
func (h *Handler) GetFavoritesStatus(ctx context.Context, favorites []FavoriteItem) []reporter.ResourceStatus {
	result := make([]reporter.ResourceStatus, len(favorites))

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
				result[i] = reporter.ResourceStatus{
					Kind:      fav.Kind,
					Name:      fav.Name,
					Namespace: fav.Namespace,
					Status:    "NotFound",
					Message:   message,
				}
				mu.Unlock()
			}

			// Workload favorites (Deployment, StatefulSet, DaemonSet, CronJob)
			// are resolved with the user's own client and a lightweight status
			// computed on the fetched object alone. This needs only get on the
			// workload, not list on pods.
			if _, ok := favoriteWorkloadKinds[fav.Kind]; ok {
				rs := h.getWorkloadFavoriteStatus(ctx, fav)
				mu.Lock()
				result[i] = rs
				mu.Unlock()
				return
			}

			gvk, err := h.preferredFluxGVK(ctx, fav.Kind)
			if err != nil {
				var message string
				switch {
				case strings.Contains(err.Error(), "no matches for kind"):
					message = "Resource kind not found in the cluster"
				default:
					message = "Internal error while fetching resource kind"
					log.FromContext(ctx).Error(err, "failed to get favorite resource kind",
						"kind", fav.Kind,
						"name", fav.Name,
						"namespace", fav.Namespace)
				}

				storeNotFound(message)
				return
			}

			obj := unstructured.Unstructured{}
			obj.SetGroupVersionKind(*gvk)

			err = h.kubeClient.GetClient(ctx).Get(ctx, client.ObjectKey{
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
					log.FromContext(ctx).Error(err, "failed to get favorite resource",
						"kind", fav.Kind,
						"name", fav.Name,
						"namespace", fav.Namespace)
				}

				storeNotFound(message)
				return
			}

			rs := reporter.NewResourceStatus(obj)
			mu.Lock()
			result[i] = rs
			mu.Unlock()
		}(i, fav)
	}

	wg.Wait()

	return result
}

// getWorkloadFavoriteStatus fetches a workload favorite with the user's own
// client and computes a lightweight rollout status from the fetched object
// alone, without listing pods, extracting images, or running SSARs. This means
// a user who can get the workload but not list its pods still sees a valid
// status instead of NotFound.
//
// It always returns a populated ResourceStatus: a "NotFound" status (with an
// explanatory message) when the workload cannot be fetched, otherwise the
// computed status. The returned Status uses the kstatus vocabulary
// (Current/InProgress/...), which the client renders via the workload badge
// helper based on the favorite's kind.
func (h *Handler) getWorkloadFavoriteStatus(ctx context.Context, fav FavoriteItem) reporter.ResourceStatus {
	notFound := func(message string) reporter.ResourceStatus {
		return reporter.ResourceStatus{
			Kind:      fav.Kind,
			Name:      fav.Name,
			Namespace: fav.Namespace,
			Status:    "NotFound",
			Message:   message,
		}
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(getWorkloadGVK(fav.Kind))

	err := h.kubeClient.GetClient(ctx).Get(ctx, client.ObjectKey{
		Namespace: fav.Namespace,
		Name:      fav.Name,
	}, obj)
	if err != nil {
		switch {
		case errors.IsNotFound(err):
			return notFound("Workload not found in the cluster")
		case errors.IsForbidden(err):
			return notFound("User does not have access to the workload")
		default:
			log.FromContext(ctx).Error(err, "failed to get favorite workload",
				"kind", fav.Kind,
				"name", fav.Name,
				"namespace", fav.Namespace)
			return notFound("Internal error while fetching workload")
		}
	}

	// Compute the rollout status (kstatus + CronJob/apps refinements).
	workloadStatus, message := computeWorkloadStatus(obj, fav.Kind)

	return reporter.ResourceStatus{
		Kind:           fav.Kind,
		Name:           fav.Name,
		Namespace:      fav.Namespace,
		Status:         workloadStatus,
		Message:        message,
		LastReconciled: metav1.Time{Time: obj.GetCreationTimestamp().Time},
	}
}
