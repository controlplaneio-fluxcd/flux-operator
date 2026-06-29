// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// InventoryObjectItem identifies a managed object to fetch.
type InventoryObjectItem struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

// InventoryObjectsRequest is the request body for POST /api/v1/inventory/objects.
type InventoryObjectsRequest struct {
	Objects []InventoryObjectItem `json:"objects"`
}

// InventoryObjectResult holds the status and sanitized manifest of one object,
// or an Error when it could not be fetched.
type InventoryObjectResult struct {
	APIVersion    string         `json:"apiVersion"`
	Kind          string         `json:"kind"`
	Namespace     string         `json:"namespace,omitempty"`
	Name          string         `json:"name"`
	Status        string         `json:"status,omitempty"`
	StatusMessage string         `json:"statusMessage,omitempty"`
	Error         string         `json:"error,omitempty"`
	Object        map[string]any `json:"object,omitempty"`
}

// InventoryObjectsHandler handles POST /api/v1/inventory/objects requests and
// returns the status and sanitized manifest of each requested object.
func (h *Handler) InventoryObjectsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var oReq InventoryObjectsRequest
	if err := json.NewDecoder(req.Body).Decode(&oReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	objects := h.GetInventoryObjects(req.Context(), oReq.Objects)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"objects": objects}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GetInventoryObjects fetches the status and sanitized manifest for each object,
// scoped to the caller's RBAC. Objects are queried in parallel with a concurrency
// limit of 4; a per-object failure is reported in its Error field instead of
// failing the whole batch.
func (h *Handler) GetInventoryObjects(ctx context.Context, items []InventoryObjectItem) []InventoryObjectResult {
	result := make([]InventoryObjectResult, len(items))

	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 4)

	for i, item := range items {
		wg.Add(1)
		go func(i int, item InventoryObjectItem) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			res := InventoryObjectResult{
				APIVersion: item.APIVersion,
				Kind:       item.Kind,
				Namespace:  item.Namespace,
				Name:       item.Name,
			}

			obj, err := h.getInventoryObject(ctx, item)
			switch {
			case err == nil:
				st := reporter.NewResourceStatus(*obj)
				res.Status = st.Status
				res.StatusMessage = st.Message
				cleanObjectForExport(obj, true)
				res.Object = obj.Object
			case errors.IsNotFound(err):
				res.Error = "NotFound"
			case errors.IsForbidden(err):
				res.Error = "Forbidden"
			default:
				res.Error = "Error"
				log.FromContext(ctx).Error(err, "failed to get inventory object",
					"apiVersion", item.APIVersion,
					"kind", item.Kind,
					"name", item.Name,
					"namespace", item.Namespace)
			}

			mu.Lock()
			result[i] = res
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()

	return result
}

// getInventoryObject fetches a single object identified by its apiVersion, kind,
// name and namespace.
func (h *Handler) getInventoryObject(ctx context.Context, item InventoryObjectItem) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(item.APIVersion)
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gv.WithKind(item.Kind))
	key := client.ObjectKey{Name: item.Name, Namespace: item.Namespace}
	if err := h.kubeClient.GetClient(ctx).Get(ctx, key, obj); err != nil {
		return nil, err
	}

	return obj, nil
}
