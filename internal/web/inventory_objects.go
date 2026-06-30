// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
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

	// StatusOnly, when true, makes the handler return each object's status and
	// message only, omitting the sanitized manifest. Callers that render status
	// without the object body (e.g. the Graph tab) use this to avoid the manifest
	// fetch overhead and payload.
	StatusOnly bool `json:"statusOnly,omitempty"`
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

	objects := h.GetInventoryObjects(req.Context(), oReq.Objects, oReq.StatusOnly)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"objects": objects}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

const (
	// maxInventoryObjects bounds the number of objects a single request may
	// query, so an oversized POST body cannot fan out into excessive API calls.
	// Objects beyond the limit are dropped rather than queried.
	maxInventoryObjects = 2000

	// inventoryObjectsWorkers is the number of concurrent object fetches.
	inventoryObjectsWorkers = 4
)

// GetInventoryObjects fetches the status and sanitized manifest for each object,
// scoped to the caller's RBAC. Objects are queried by a fixed pool of
// inventoryObjectsWorkers, so the number of goroutines stays constant regardless
// of the request size. A per-object failure is reported in its Error field
// instead of failing the whole batch. When statusOnly is true, the sanitized
// manifest is omitted and only the status and message are returned.
func (h *Handler) GetInventoryObjects(ctx context.Context, items []InventoryObjectItem, statusOnly bool) []InventoryObjectResult {
	// Cap the batch size so a large request cannot fan out into excessive API
	// calls; the surplus items are dropped rather than queried.
	if len(items) > maxInventoryObjects {
		log.FromContext(ctx).Info("inventory objects request truncated to the maximum batch size",
			"requested", len(items), "limit", maxInventoryObjects)
		items = items[:maxInventoryObjects]
	}

	result := make([]InventoryObjectResult, len(items))

	// Fixed worker pool: each index is handled by exactly one worker, so the
	// writes to result[i] need no locking.
	work := make(chan int)
	var wg sync.WaitGroup
	for range inventoryObjectsWorkers {
		wg.Go(func() {
			for i := range work {
				result[i] = h.inventoryObjectResult(ctx, items[i], statusOnly)
			}
		})
	}
	for i := range items {
		work <- i
	}
	close(work)
	wg.Wait()

	return result
}

// inventoryObjectResult fetches and assembles the result for a single inventory
// item, scoped to the caller's RBAC. A fetch failure is reported in the Error
// field; a panic during status computation or sanitization is recovered.
func (h *Handler) inventoryObjectResult(ctx context.Context, item InventoryObjectItem, statusOnly bool) (res InventoryObjectResult) {
	res = InventoryObjectResult{
		APIVersion: item.APIVersion,
		Kind:       item.Kind,
		Namespace:  item.Namespace,
		Name:       item.Name,
	}

	defer func() {
		if r := recover(); r != nil {
			res.Object = nil
			res.Error = "Error"
			log.FromContext(ctx).Error(fmt.Errorf("panic: %v", r), "recovered while processing inventory object",
				"apiVersion", item.APIVersion,
				"kind", item.Kind,
				"name", item.Name,
				"namespace", item.Namespace)
		}
	}()

	obj, err := h.getInventoryObject(ctx, item)
	switch {
	case err == nil:
		res.Status, res.StatusMessage = computeObjectStatus(obj)
		if !statusOnly {
			cleanObjectForExport(obj, true)
			res.Object = obj.Object
		}
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

	return res
}

// getInventoryObject fetches a single object identified by its
// apiVersion, kind, name, and namespace.
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

// computeObjectStatus returns an object's status and message, never failing:
//   - Flux and Flux Operator kinds use the Ready-condition reader.
//   - Workloads (Deployment/StatefulSet/DaemonSet/CronJob) use the workload status
//     logic (kstatus + CronJob/apps refinements), matching the Workloads tab.
//   - Every other kind uses kstatus; an object it cannot assess yields "Unknown".
func computeObjectStatus(obj *unstructured.Unstructured) (string, string) {
	switch {
	case fluxcdv1.IsFluxAPI(obj.GetAPIVersion()):
		rs := reporter.NewResourceStatus(*obj)
		return rs.Status, rs.Message
	case isWorkloadObject(obj):
		return computeWorkloadStatus(obj, obj.GetKind())
	default:
		res, err := status.Compute(obj)
		if err != nil {
			return reporter.StatusUnknown, fmt.Sprintf("Failed to compute status: %s", err.Error())
		}
		return string(res.Status), res.Message
	}
}
