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
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ResourcesHandler handles GET /api/v1/resources requests and returns the status of Flux resources.
// Supports optional query parameters: kind, name, namespace, status
// Example: /api/v1/resources?kind=FluxInstance&name=flux&namespace=flux-system&status=Ready
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
	status := queryParams.Get("status")

	// Build kinds array based on query parameter
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Default kinds
		kinds = []string{
			// Appliers
			fluxcdv1.ResourceSetKind,
			fluxcdv1.ResourceSetInputProviderKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
			// Sources
			fluxcdv1.FluxGitRepositoryKind,
			fluxcdv1.FluxOCIRepositoryKind,
			fluxcdv1.FluxHelmChartKind,
			fluxcdv1.FluxHelmRepositoryKind,
			fluxcdv1.FluxBucketKind,
			fluxcdv1.FluxArtifactGeneratorKind,
		}
	}

	// Prepare list of namespaces to search in
	var namespaces []string

	// If namespace is specified, use it directly
	if namespace != "" {
		namespaces = []string{namespace}
	} else {
		// Check if the user has access to all namespaces
		userNamespaces, all, err := r.kubeClient.ListUserNamespaces(req.Context())
		if err != nil {
			r.log.Error(err, "failed to list user namespaces", "url", req.URL.String())
			if apierrors.IsForbidden(err) {
				http.Error(w, err.Error(), http.StatusForbidden)
			} else {
				http.Error(w, "failed to list user namespaces", http.StatusInternalServerError)
			}
			return
		}

		// If the user has no access to any namespaces, return empty result
		if len(userNamespaces) == 0 {
			w.Header().Set("Content-Type", "application/json")
			response := map[string]any{"resources": []ResourceStatus{}}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
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

	// Get resource status from the cluster using the request context
	resources, err := r.GetResourcesStatus(req.Context(), kinds, name, namespaces, status, 2500)
	if err != nil {
		r.log.Error(err, "failed to get resources status", "url", req.URL.String(),
			"kind", kind, "name", name, "namespace", namespace, "status", status)
		if apierrors.IsForbidden(err) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		// Return empty array instead of error for better UX
		resources = []ResourceStatus{}
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
}

// GetResourcesStatus returns the status for the specified resource kinds and optional name in the given namespace.
// If name is empty, returns the status for all resources of the specified kinds are returned.
// Filters by status (Ready, Failed, Progressing, Suspended, Unknown) if provided.
func (r *Router) GetResourcesStatus(ctx context.Context, kinds []string, name string, namespaces []string, status string, matchLimit int) ([]ResourceStatus, error) {
	var result []ResourceStatus

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
	errChan := make(chan error, len(kinds))

	for _, kind := range kinds {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()

			gvk, err := r.preferredFluxGVK(ctx, kind)
			if err != nil {
				if strings.Contains(err.Error(), "no matches for kind") {
					return
				}
				errChan <- fmt.Errorf("unable to get gvk for kind %s : %w", kind, err)
				return
			}

			// Determine which namespaces to query.
			// If namespaces is empty, query all namespaces (cluster-wide access).
			// Otherwise, query each namespace in the list.
			namespacesToQuery := namespaces
			if len(namespacesToQuery) == 0 {
				namespacesToQuery = []string{""}
			}

			var byKindResult []ResourceStatus
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

				if err := r.kubeClient.GetClient(ctx).List(ctx, &list, listOpts...); err != nil {
					errChan <- fmt.Errorf("unable to list resources for kind %s in namespace %s: %w", kind, ns, err)
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

					rs := r.resourceStatusFromUnstructured(obj)
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
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	// Filter by status if specified
	if status != "" {
		filteredResult := make([]ResourceStatus, 0, len(result))
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

// resourceStatusFromUnstructured extracts the ResourceStatus from an unstructured Kubernetes object.
// Maps Kubernetes condition status to one of: "Ready", "Failed", "Progressing", "Suspended", "Unknown"
// nolint: gocyclo
func (r *Router) resourceStatusFromUnstructured(obj unstructured.Unstructured) ResourceStatus {
	status := StatusUnknown
	message := "No status information available"
	lastReconciled := metav1.Time{Time: obj.GetCreationTimestamp().Time}

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

	// if kind is HelmRepository and has .spec.type of 'oci', set status to Ready
	if obj.GetKind() == fluxcdv1.FluxHelmRepositoryKind {
		if specType, found, err := unstructured.NestedString(obj.Object, "spec", "type"); found && err == nil {
			if specType == "oci" && status == StatusUnknown {
				status = StatusReady
				message = "Valid configuration"
			}
		}
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

	return rs
}
