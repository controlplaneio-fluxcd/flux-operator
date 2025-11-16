// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourcesHandler handles GET /api/v1/resources requests and returns the status of Flux resources.
// Supports optional query parameters: kind, name, namespace
// Example: /api/v1/resources?kind=FluxInstance&name=flux&namespace=flux-system
func (r *Router) ResourcesHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	kind := queryParams.Get("kind")
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")

	// Build kinds array based on query parameter
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Default kinds
		kinds = []string{
			// Appliers
			fluxcdv1.FluxInstanceKind,
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
			// Sources
			fluxcdv1.FluxGitRepositoryKind,
			fluxcdv1.FluxOCIRepositoryKind,
			fluxcdv1.FluxHelmChartKind,
			fluxcdv1.FluxArtifactGeneratorKind,
		}
	}

	// Get resource status from the cluster using the request context
	resources, err := r.GetResourcesStatus(req.Context(), kinds, name, namespace)
	if err != nil {
		r.log.Error(err, "failed to get resources status", "url", req.URL.String(),
			"kind", kind, "name", name, "namespace", namespace)
		// Return empty array instead of error for better UX
		resources = []ResourceStatus{}
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Encode and send the response
	response := map[string]any{"resources": resources}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

const (
	StatusReady       = "Ready"
	StatusFailed      = "Failed"
	StatusProgressing = "Progressing"
	StatusSuspended   = "Suspended"
	StatusUnknown     = "Unknown"
)

// ResourceStatus represents the reconciliation status of a Flux resource.
type ResourceStatus struct {
	// Name of the resource.
	Name string `json:"name"`

	// Kind of the resource.
	Kind string `json:"kind"`

	// Namespace of the resource.
	Namespace string `json:"namespace"`

	// Status can be "Ready", "Failed", "Progressing", "Suspended", "Unknown"
	Status string `json:"status"`

	// Reason is a brief reason for the current status.
	Message string `json:"message"`

	// LastReconciled is the timestamp of the last reconciliation.
	LastReconciled metav1.Time `json:"lastReconciled"`

	// Inventory holds the list of managed resources.
	Inventory []InventoryEntry `json:"inventory,omitempty"`
}

// GetResourcesStatus returns the status for the specified resource kinds and optional name in the given namespace.
// If name is empty, returns the status for all resources of the specified kinds are returned.
func (r *Router) GetResourcesStatus(ctx context.Context, kinds []string, name, namespace string) ([]ResourceStatus, error) {
	var result []ResourceStatus

	if len(kinds) == 0 {
		return nil, errors.New("no resource kinds specified")
	}

	// Set limit based on number of kinds
	limit := 500
	if len(kinds) > 1 {
		limit = 100
	}

	// Query resources for each kind in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(kinds))

	for _, kind := range kinds {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()

			gvk, err := r.preferredFluxGVK(kind)
			if err != nil {
				if strings.Contains(err.Error(), "no matches for kind") {
					return
				}
				errChan <- fmt.Errorf("unable to get gvk for kind %s : %w", kind, err)
				return
			}

			list := unstructured.UnstructuredList{
				Object: map[string]any{
					"apiVersion": gvk.Group + "/" + gvk.Version,
					"kind":       gvk.Kind,
				},
			}

			listOpts := []client.ListOption{
				client.Limit(limit),
			}
			if namespace != "" {
				listOpts = append(listOpts, client.InNamespace(namespace))
			}

			// Add name filter if provided and doesn't contain wildcards
			if name != "" && !hasWildcard(name) {
				listOpts = append(listOpts, client.MatchingFields{"metadata.name": name})
			}

			if err := r.kubeClient.List(ctx, &list, listOpts...); err != nil {
				errChan <- fmt.Errorf("unable to list resources for kind %s : %w", kind, err)
				return
			}

			mu.Lock()
			for _, obj := range list.Items {
				// Filter by name using wildcard matching if needed
				if hasWildcard(name) {
					objName, _, _ := unstructured.NestedString(obj.Object, "metadata", "name")
					if !matchesWildcard(objName, name) {
						continue
					}
				}

				rs := r.resourceStatusFromUnstructured(ctx, obj)
				result = append(result, rs)
			}
			mu.Unlock()
		}(kind)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return result, nil
}

// resourceStatusFromUnstructured extracts the ResourceStatus from an unstructured Kubernetes object.
// Maps Kubernetes condition status to one of: "Ready", "Failed", "Progressing", "Suspended", "Unknown"
func (r *Router) resourceStatusFromUnstructured(ctx context.Context, obj unstructured.Unstructured) ResourceStatus {
	status := StatusUnknown
	message := "No status information available"
	lastReconciled := metav1.Now()

	// Check for status conditions (Ready condition)
	if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
		for _, cond := range conditions {
			if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
				// Get condition status (True/False/Unknown)
				if condStatus, ok := condition["status"].(string); ok {
					switch condStatus {
					case "True":
						status = StatusReady
					case "False":
						status = StatusFailed
					case "Unknown":
						// Check reason to determine if it's progressing or truly unknown
						if reason, exists := condition["reason"]; exists {
							reasonStr, _ := reason.(string)
							if reasonStr == StatusProgressing || reasonStr == "Reconciling" {
								status = StatusProgressing
							} else {
								status = StatusUnknown
							}
						} else {
							status = StatusProgressing
						}
					default:
						// Any other status value defaults to Unknown
						status = StatusUnknown
					}
				}

				// Extract message
				if msg, exists := condition["message"]; exists {
					if msgStr, ok := msg.(string); ok && msgStr != "" {
						message = msgStr
					}
				}

				// Extract last transition time
				if lastTransitionTime, exists := condition["lastTransitionTime"]; exists {
					if timeStr, ok := lastTransitionTime.(string); ok {
						if parsedTime, err := time.Parse(time.RFC3339, timeStr); err == nil {
							lastReconciled = metav1.Time{Time: parsedTime}
						}
					}
				}

				break // Found Ready condition, no need to check others
			}
		}
	}

	// If kind is Alert or Provider set status to Ready as they don't have conditions
	if (obj.GetKind() == fluxcdv1.FluxAlertKind ||
		obj.GetKind() == fluxcdv1.FluxAlertProviderKind) &&
		status == StatusUnknown {
		status = StatusReady
		message = "Valid configuration"
	}

	// Check for suspended state (takes precedence over condition status)
	// Check reconcile annotation
	if ssautil.AnyInMetadata(&obj,
		map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
		status = StatusSuspended
		message = "Reconciliation suspended"
	}

	// Check spec.suspend field
	if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
		status = StatusSuspended
		message = "Reconciliation suspended"
	}

	rs := ResourceStatus{
		Kind:           obj.GetKind(),
		Name:           obj.GetName(),
		Namespace:      obj.GetNamespace(),
		LastReconciled: lastReconciled,
		Status:         status,
		Message:        message,
	}

	// Extract inventory of managed resources
	inventory := r.getInventory(ctx, obj)
	if len(inventory) > 0 {
		rs.Inventory = inventory
	}
	return rs
}
