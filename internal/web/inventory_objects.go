// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/runtime/cel"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	// Owner optionally identifies the Flux Kustomization or HelmRelease whose
	// inventory the objects belong to. When set, the owner's
	// spec.healthCheckExprs are evaluated for the objects they match, so the
	// reported status agrees with the verdict the Flux controller computes for
	// the owner's Ready condition.
	Owner *InventoryObjectItem `json:"owner,omitempty"`
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

	evaluators := h.getOwnerHealthCheckEvaluators(req.Context(), oReq.Owner)
	objects := h.GetInventoryObjects(req.Context(), oReq.Objects, oReq.StatusOnly, evaluators)

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
// manifest is omitted and only the status and message are returned. The
// evaluators, built from the owner's spec.healthCheckExprs, take precedence
// over the builtin status logic for the kinds they match.
func (h *Handler) GetInventoryObjects(ctx context.Context, items []InventoryObjectItem, statusOnly bool, evaluators healthCheckEvaluators) []InventoryObjectResult {
	// Cap the batch size so a large request cannot fan out into excessive API
	// calls; the surplus items are dropped rather than queried.
	if len(items) > maxInventoryObjects {
		log.FromContext(ctx).Info("inventory objects request truncated to the maximum batch size",
			"requested", len(items), "limit", maxInventoryObjects)
	}

	return processBatch(items, maxInventoryObjects, inventoryObjectsWorkers, func(item InventoryObjectItem) InventoryObjectResult {
		return h.inventoryObjectResult(ctx, item, statusOnly, evaluators)
	})
}

// healthCheckEvaluators maps a GroupKind to the CEL status evaluator declared
// for it in a Kustomization or HelmRelease spec.healthCheckExprs. A key with an
// empty Kind matches every kind in the group, mirroring the Flux semantics.
type healthCheckEvaluators map[schema.GroupKind]*cel.StatusEvaluator

// lookup returns the evaluator matching the GroupKind, preferring an exact
// kind match over a group-wide one, or nil when none is declared.
func (e healthCheckEvaluators) lookup(gk schema.GroupKind) *cel.StatusEvaluator {
	if ev, ok := e[gk]; ok {
		return ev
	}
	return e[schema.GroupKind{Group: gk.Group}]
}

// getOwnerHealthCheckEvaluators fetches the owner with the caller's client and
// compiles its spec.healthCheckExprs. It returns nil when no owner is given,
// the owner is not a Flux Kustomization or HelmRelease (the only kinds that
// declare health check expressions), or it cannot be read, so the caller
// falls back to the builtin status logic.
func (h *Handler) getOwnerHealthCheckEvaluators(ctx context.Context, owner *InventoryObjectItem) healthCheckEvaluators {
	if owner == nil || !fluxcdv1.IsFluxAPI(owner.APIVersion) {
		return nil
	}
	if owner.Kind != fluxcdv1.FluxKustomizationKind && owner.Kind != fluxcdv1.FluxHelmReleaseKind {
		return nil
	}

	obj, err := h.getInventoryObject(ctx, *owner)
	if err != nil {
		if !errors.IsNotFound(err) && !errors.IsForbidden(err) {
			log.FromContext(ctx).Error(err, "failed to get inventory owner",
				"apiVersion", owner.APIVersion,
				"kind", owner.Kind,
				"name", owner.Name,
				"namespace", owner.Namespace)
		}
		return nil
	}

	return newHealthCheckEvaluators(ctx, obj)
}

// newHealthCheckEvaluators compiles the spec.healthCheckExprs of a Kustomization
// or HelmRelease into evaluators keyed by GroupKind. It returns nil when the
// owner declares no expressions. An entry that cannot be decoded or compiled is
// logged and skipped rather than failing the whole batch.
func newHealthCheckEvaluators(ctx context.Context, owner *unstructured.Unstructured) healthCheckEvaluators {
	raw, found, err := unstructured.NestedSlice(owner.Object, "spec", "healthCheckExprs")
	if err != nil || !found || len(raw) == 0 {
		return nil
	}

	evaluators := make(healthCheckEvaluators, len(raw))
	for i, item := range raw {
		var hc kustomize.CustomHealthCheck
		m, ok := item.(map[string]any)
		if !ok {
			log.FromContext(ctx).Error(fmt.Errorf("expected an object, got %T", item),
				"failed to decode owner health check expressions",
				"kind", owner.GetKind(),
				"name", owner.GetName(),
				"namespace", owner.GetNamespace(),
				"index", i)
			continue
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &hc); err != nil {
			log.FromContext(ctx).Error(err, "failed to decode owner health check expressions",
				"kind", owner.GetKind(),
				"name", owner.GetName(),
				"namespace", owner.GetNamespace(),
				"index", i)
			continue
		}
		ev, err := cel.NewStatusEvaluator(&hc.HealthCheckExpressions)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to compile owner health check expressions",
				"kind", owner.GetKind(),
				"name", owner.GetName(),
				"namespace", owner.GetNamespace(),
				"index", i)
			continue
		}
		evaluators[schema.FromAPIVersionAndKind(hc.APIVersion, hc.Kind).GroupKind()] = ev
	}

	if len(evaluators) == 0 {
		return nil
	}
	return evaluators
}

// inventoryObjectResult fetches and assembles the result for a single inventory
// item, scoped to the caller's RBAC. A fetch failure is reported in the Error
// field; a panic during status computation or sanitization is recovered.
func (h *Handler) inventoryObjectResult(ctx context.Context, item InventoryObjectItem, statusOnly bool, evaluators healthCheckEvaluators) (res InventoryObjectResult) {
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
		res.Status, res.StatusMessage = computeObjectStatus(ctx, obj, evaluators)
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
//   - Kinds matched by the owner's spec.healthCheckExprs use the CEL evaluator,
//     taking precedence over the builtin logic as in the Flux controllers.
//   - Flux and Flux Operator kinds use the Ready-condition reader.
//   - Workloads (Deployment/StatefulSet/DaemonSet/CronJob) use the workload status
//     logic (kstatus + CronJob/apps refinements), matching the Workloads tab.
//   - Every other kind uses kstatus; an object it cannot assess yields "Unknown".
func computeObjectStatus(ctx context.Context, obj *unstructured.Unstructured, evaluators healthCheckEvaluators) (string, string) {
	if ev := evaluators.lookup(obj.GroupVersionKind().GroupKind()); ev != nil {
		res, err := ev.Evaluate(ctx, obj)
		if err != nil {
			return reporter.StatusUnknown, fmt.Sprintf("Failed to compute status: %s", err.Error())
		}
		return string(res.Status), string(res.Status)
	}

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
