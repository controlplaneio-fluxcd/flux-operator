// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// SearchHandler handles GET /api/v1/search requests and returns the status of Flux resources
// from the cached search index. Results are filtered by name with wildcard support.
// Example: /api/v1/search?name=flux
func (h *Handler) SearchHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	queryParams := req.URL.Query()
	name := queryParams.Get("name")
	namespace := queryParams.Get("namespace")
	kind := queryParams.Get("kind")

	// if name does not contain wildcard, append * to perform partial match
	if name != "" && !hasWildcard(name) {
		name = "*" + name + "*"
	}

	// Query the cached search index with RBAC filtering.
	resources := h.GetCachedResources(req.Context(), kind, name, namespace, "", 10)

	// Strip message from results, not needed for search.
	for i := range resources {
		resources[i].Message = ""
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

// GetCachedResources returns resources from the cached search index filtered by the given criteria.
// If name and namespace filters are empty, it will return resources across all namespaces (subject to RBAC).
func (h *Handler) GetCachedResources(ctx context.Context, kind, name, namespace, status string, limit int) []reporter.ResourceStatus {
	// Get user-visible namespaces for RBAC filtering.
	userNamespaces, allNamespaces, err := h.kubeClient.ListUserNamespaces(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to list user namespaces for cached resources")
		return []reporter.ResourceStatus{}
	}

	// If the user has no access to any namespace, return empty results.
	if !allNamespaces && len(userNamespaces) == 0 {
		return []reporter.ResourceStatus{}
	}

	// For cluster-wide access, pass nil (no RBAC filtering).
	// Otherwise, pass the user's namespace list.
	var allowedNamespaces []string
	if !allNamespaces {
		allowedNamespaces = userNamespaces
	}

	return h.searchIndex.SearchResources(allowedNamespaces, kind, name, namespace, status, limit)
}

// hasWildcard returns true if the pattern contains wildcard characters.
func hasWildcard(pattern string) bool {
	return pattern != "" && strings.Contains(pattern, "*")
}

// matchesWildcard checks if a name matches a pattern with wildcard support.
// Supports * (matches any characters). If no wildcards present, does exact match.
// Matching is case-insensitive.
func matchesWildcard(name, pattern string) bool {
	name = strings.ToLower(name)
	pattern = strings.ToLower(pattern)

	// If no wildcards, do exact match
	if !strings.Contains(pattern, "*") {
		return name == pattern
	}

	// Split pattern by * and check each segment appears in order
	segments := strings.Split(pattern, "*")
	pos := 0

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		idx := strings.Index(name[pos:], segment)
		if idx == -1 {
			return false
		}

		// First segment must be at the start (unless pattern starts with *)
		if i == 0 && idx != 0 {
			return false
		}

		pos += idx + len(segment)
	}

	// Last segment must be at the end (unless pattern ends with *)
	if len(segments) > 0 && segments[len(segments)-1] != "" && pos != len(name) {
		return false
	}

	return true
}
