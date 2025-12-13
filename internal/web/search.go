// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// SearchHandler handles GET /api/v1/search requests and returns the status of Flux resources.
// Results are filtered by name with wildcard support.
// Example: /api/v1/search?&name=flux
func (r *Router) SearchHandler(w http.ResponseWriter, req *http.Request) {
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
	var kinds []string
	if kind != "" {
		kinds = []string{kind}
	} else {
		// Limit search to applier kinds
		kinds = []string{
			fluxcdv1.FluxInstanceKind,
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.FluxHelmReleaseKind,
		}
	}

	// If namespace is specified, add sources kinds as well
	if namespace != "" && kind == "" {
		kinds = append(kinds,
			fluxcdv1.FluxGitRepositoryKind,
			fluxcdv1.FluxOCIRepositoryKind,
			fluxcdv1.FluxHelmChartKind,
			fluxcdv1.FluxHelmRepositoryKind,
			fluxcdv1.FluxBucketKind,
			fluxcdv1.FluxArtifactGeneratorKind,
		)
	}

	// Prepare list of namespaces to search in
	var namespaces []string
	if namespace != "" {
		namespaces = []string{namespace}
	} else {
		// Check if the user has access to all namespaces
		userNamespaces, all, err := r.kubeClient.ListUserNamespaces(req.Context())
		if err != nil {
			r.log.Error(err, "failed to list user namespaces", "url", req.URL.String())
			if errors.IsForbidden(err) {
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

		// If the user does not have access to all namespaces, limit search to their namespaces
		if !all {
			namespaces = userNamespaces
		}
	}

	// Get resource status from the cluster using the request context
	resources, err := r.GetResourcesStatus(req.Context(), kinds, name, namespaces, "", 10)
	if err != nil {
		r.log.Error(err, "failed to get resources status", "url", req.URL.String(), "name", name, "namespace", namespace)
		if errors.IsForbidden(err) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
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
