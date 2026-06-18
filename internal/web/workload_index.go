// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/reporter"
)

// WorkloadIndex holds cached workload data built from the reporter's Compute()
// results. Workloads are extracted from Flux applier inventories and carry the
// owning reconciler's reference and status (badge only).
type WorkloadIndex struct {
	mu        sync.RWMutex
	workloads []reporter.WorkloadRef
	updatedAt time.Time
}

// Update sorts and stores the given workloads in the index, replacing any
// existing data.
func (idx *WorkloadIndex) Update(workloads []reporter.WorkloadRef) {
	sorted := buildWorkloadIndex(workloads)
	idx.mu.Lock()
	idx.workloads = sorted
	idx.updatedAt = time.Now()
	idx.mu.Unlock()
}

// SearchWorkloads filters indexed workloads by the given criteria.
// allowedNamespaces restricts results to RBAC-visible namespaces:
// nil means no RBAC filtering (cluster-wide access),
// non-nil (including empty) means only return workloads in those namespaces.
// kind filters by exact match (empty means all kinds).
// name filters by wildcard match using matchesWildcard() (empty means all names).
// namespace filters by exact match (empty means all namespaces).
// limit caps the number of returned results (0 means unlimited).
// Results are sorted by LastReconciled (newest first), with a (namespace, name,
// kind) tiebreaker since all workloads under one applier share the applier timestamp.
func (idx *WorkloadIndex) SearchWorkloads(allowedNamespaces []string, kind, name, namespace string, limit int) []reporter.WorkloadRef {
	idx.mu.RLock()
	workloads := idx.workloads
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

	result := []reporter.WorkloadRef{}
	for _, wl := range workloads {
		// Filter by RBAC-visible namespaces.
		if nsAllowed != nil {
			if _, ok := nsAllowed[wl.Namespace]; !ok {
				continue
			}
		}

		// Filter by namespace (exact match).
		if namespace != "" && wl.Namespace != namespace {
			continue
		}

		// Filter by kind (exact match).
		if kind != "" && wl.Kind != kind {
			continue
		}

		// Filter by name (wildcard match).
		if name != "" && !matchesWildcard(wl.Name, name) {
			continue
		}

		result = append(result, wl)
	}

	// Sort by LastReconciled (newest first), with a (namespace, name, kind)
	// tiebreaker for stable ordering, then truncate to limit. Kind is included
	// because two workloads of different kinds can share a namespace and name.
	sort.Slice(result, func(i, j int) bool {
		if !result[i].LastReconciled.Time.Equal(result[j].LastReconciled.Time) {
			return result[i].LastReconciled.Time.After(result[j].LastReconciled.Time)
		}
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Kind < result[j].Kind
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// buildWorkloadIndex normalises workload names for case-insensitive matching
// and returns a stable-sorted copy suitable for the WorkloadIndex.
func buildWorkloadIndex(workloads []reporter.WorkloadRef) []reporter.WorkloadRef {
	if len(workloads) == 0 {
		return nil
	}

	out := make([]reporter.WorkloadRef, len(workloads))
	copy(out, workloads)

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
