// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// SearchIndex holds cached search data built from the reporter's Compute() results.
type SearchIndex struct {
	mu        sync.RWMutex
	resources []reporter.ResourceStatus
	updatedAt time.Time
}

// Update replaces the indexed resources and sets the updated timestamp.
func (idx *SearchIndex) Update(resources []reporter.ResourceStatus) {
	idx.mu.Lock()
	idx.resources = resources
	idx.updatedAt = time.Now()
	idx.mu.Unlock()
}

// SearchResources filters indexed resources by the given criteria.
// allowedNamespaces restricts results to RBAC-visible namespaces:
// nil means no RBAC filtering (cluster-wide access),
// non-nil (including empty) means only return resources in those namespaces.
// kind filters by exact match (empty means all kinds).
// name filters by wildcard match using matchesWildcard() (empty means all names).
// namespace filters by exact match (empty means all namespaces).
// limit caps the number of returned results (0 means unlimited).
// Results are sorted by LastReconciled (newest first).
func (idx *SearchIndex) SearchResources(allowedNamespaces []string, kind, name, namespace string, limit int) []reporter.ResourceStatus {
	idx.mu.RLock()
	resources := idx.resources
	idx.mu.RUnlock()

	// Build namespace allowlist for fast lookup.
	// nil allowedNamespaces = cluster-wide access (no filtering).
	// Non-nil (including empty) = only allow listed namespaces.
	var nsAllowed map[string]struct{}
	if allowedNamespaces != nil {
		nsAllowed = make(map[string]struct{}, len(allowedNamespaces))
		for _, ns := range allowedNamespaces {
			nsAllowed[ns] = struct{}{}
		}
	}

	// Lowercase the name pattern once before the loop so that
	// matchesWildcard's per-item ToLower is a no-op on pre-normalized index data.
	name = strings.ToLower(name)

	result := []reporter.ResourceStatus{}
	for _, rs := range resources {
		// Filter by RBAC-visible namespaces.
		if nsAllowed != nil {
			if _, ok := nsAllowed[rs.Namespace]; !ok {
				continue
			}
		}

		// Filter by namespace (exact match).
		if namespace != "" && rs.Namespace != namespace {
			continue
		}

		// Filter by kind (exact match).
		if kind != "" && rs.Kind != kind {
			continue
		}

		// Filter by name (wildcard match).
		if name != "" && !matchesWildcard(rs.Name, name) {
			continue
		}

		result = append(result, rs)
	}

	// Sort by LastReconciled (newest first), then truncate to limit.
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastReconciled.Time.After(result[j].LastReconciled.Time)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// buildSearchIndex normalises resource names for case-insensitive matching
// and returns a stable-sorted copy suitable for the SearchIndex.
func buildSearchIndex(resources []reporter.ResourceStatus) []reporter.ResourceStatus {
	if len(resources) == 0 {
		return nil
	}

	out := make([]reporter.ResourceStatus, len(resources))
	copy(out, resources)

	for i := range out {
		out[i].Name = strings.ToLower(out[i].Name)
	}

	// Sort by kind, namespace, then name for stable ordering.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})

	return out
}
