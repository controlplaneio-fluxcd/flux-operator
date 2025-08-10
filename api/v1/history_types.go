// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"fmt"
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// MaxHistorySize defines the maximum number of snapshots to keep in history.
	MaxHistorySize = 5
)

// History represents a collection of snapshots that tracks the reconciliation
// history of a group of resources, automatically sorted by LastReconciled timestamp.
type History []Snapshot

// Len returns the length of the history slice.
func (h History) Len() int { return len(h) }

// Less reports whether the element with index i should sort before the element with index j.
// Sorts by LastReconciled in descending order (most recent first).
func (h History) Less(i, j int) bool {
	return h[i].LastReconciled.After(h[j].LastReconciled.Time)
}

// Swap swaps the elements with indexes i and j.
func (h History) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Latest returns the most recent snapshot from the history.
// Returns nil if the history is empty.
func (h History) Latest() *Snapshot {
	if len(h) == 0 {
		return nil
	}
	sort.Sort(h)
	return &h[0]
}

// Truncate keeps only the latest snapshots in the history up to MaxHistorySize.
// The history is sorted before truncation to ensure the most recent snapshots are kept.
func (h *History) Truncate() {
	if len(*h) <= MaxHistorySize {
		return
	}
	sort.Sort(*h)
	*h = (*h)[:MaxHistorySize]
}

// Upsert adds a new snapshot to the history or updates an existing one
// with the same digest. The history is reordered by LastReconciled after the operation.
// When adding new snapshots, the history is automatically truncated to MaxHistorySize.
func (h *History) Upsert(digest string, timestamp time.Time, duration time.Duration, status string, metadata map[string]string) {
	now := metav1.NewTime(timestamp)
	durationMeta := metav1.Duration{Duration: duration}

	// Look for existing snapshot with same digest
	for i := range *h {
		if (*h)[i].Digest == digest {
			// Update existing snapshot
			(*h)[i].LastReconciled = now
			(*h)[i].LastReconciledDuration = durationMeta
			(*h)[i].LastReconciledStatus = status
			(*h)[i].Metadata = metadata
			// Sort and return
			sort.Sort(*h)
			return
		}
	}

	// Add new snapshot
	newSnapshot := Snapshot{
		Digest:                 digest,
		FirstReconciled:        now,
		LastReconciled:         now,
		LastReconciledDuration: durationMeta,
		LastReconciledStatus:   status,
		Metadata:               metadata,
	}

	*h = append(*h, newSnapshot)
	h.Truncate()
}

// Snapshot represents a point-in-time record of a group of resources reconciliation,
// including timing information, status, and a unique digest identifier.
type Snapshot struct {
	// Digest is the checksum in the format `<algo>:<hex>` of the resources in this snapshot.
	// It acts as a unique identifier for the snapshot.
	// +required
	Digest string `json:"digest"`

	// FirstReconciled is the time when this revision was first reconciled to the cluster.
	// +required
	FirstReconciled metav1.Time `json:"firstReconciled"`

	// LastReconciled is the time when this revision was last reconciled to the cluster.
	// It acts as the sorting key for the history.
	// +required
	LastReconciled metav1.Time `json:"lastReconciled"`

	// LastReconciledDuration is time it took to reconcile the resources in this revision.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +required
	LastReconciledDuration metav1.Duration `json:"lastReconciledDuration"`

	// LastReconciledStatus is the status of the last reconciliation.
	// +required
	LastReconciledStatus string `json:"lastReconciledStatus"`

	// Metadata contains additional information about the snapshot.
	// Consumers can add useful information here, such as revision numbers,
	// resource counts, or other reconciler-specific data.
	// +optional
	Metadata map[string]string `json:"metadata,omitempty"`
}

// String returns a human-readable representation of the snapshot.
func (s Snapshot) String() string {
	id := s.Digest
	if idx := strings.Index(id, ":"); idx != -1 && len(id) > idx+8 {
		id = id[idx+1 : idx+9] // Skip algo prefix, take 8 hex chars
	}
	return fmt.Sprintf("id=%s last-reconciled=%s duration=%s status=%s",
		id,
		s.LastReconciled.Format(time.RFC3339),
		s.LastReconciledDuration.Duration.String(),
		s.LastReconciledStatus)
}
