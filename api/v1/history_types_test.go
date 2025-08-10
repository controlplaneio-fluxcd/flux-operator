// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHistoryUpsert(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *History
		digest   string
		status   string
		expected int // expected length after upsert
	}{
		{
			name: "add new snapshot to empty history",
			setup: func() *History {
				h := &History{}
				return h
			},
			digest:   "sha256:abc123",
			status:   "Success",
			expected: 1,
		},
		{
			name: "update existing snapshot",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
					},
				}
				return h
			},
			digest:   "sha256:abc123",
			status:   "Success",
			expected: 1,
		},
		{
			name: "add new snapshot due to different status",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
					},
				}
				return h
			},
			digest:   "sha256:abc123",
			status:   "Failure",
			expected: 2,
		},
		{
			name: "add new snapshot to existing history",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:def456",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
					},
				}
				return h
			},
			digest:   "sha256:abc123",
			status:   "Success",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.setup()
			metadata := map[string]string{"test": "value"}

			h.Upsert(tt.digest, time.Now(), 45*time.Second, tt.status, metadata)

			if len(*h) != tt.expected {
				t.Errorf("expected length %d, got %d", tt.expected, len(*h))
			}

			// Verify latest entry matches the upserted snapshot
			if h.Latest().Digest != tt.digest && h.Latest().LastReconciledStatus != tt.status {
				t.Errorf("expected first snapshot digest %s status %s, got %s %s",
					tt.digest, tt.status, h.Latest().Digest, h.Latest().LastReconciledStatus)
			}

			// Verify metadata was set
			for _, snapshot := range *h {
				if snapshot.Digest == tt.digest && snapshot.LastReconciledStatus == tt.status {
					if snapshot.Metadata["test"] != "value" {
						t.Errorf("expected metadata 'test'='value', got %v", snapshot.Metadata)
					}
				}
			}
		})
	}
}

func TestHistoryTruncate(t *testing.T) {
	// Create history with more than MaxHistorySize snapshots
	h := &History{}
	baseTime := time.Now()

	// Add 7 snapshots (more than MaxHistorySize=5)
	digests := make([]string, 7)
	for i := 0; i < 7; i++ {
		digests[i] = "sha256:digest" + string(rune('0'+i))
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		h.Upsert(digests[i], timestamp, 30*time.Second, "Success", nil)
	}

	// Should be truncated to 5
	if len(*h) != MaxHistorySize {
		t.Errorf("expected length %d after truncation, got %d", MaxHistorySize, len(*h))
	}

	// Verify it kept the most recent ones
	latest := (*h)[0]
	expectedLatestDigest := digests[6] // Last one added (index 6)
	if latest.Digest != expectedLatestDigest {
		t.Errorf("expected latest digest %s, got %s", expectedLatestDigest, latest.Digest)
	}
}

func TestHistoryLatest(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() History
		expected *Snapshot
	}{
		{
			name: "empty history",
			setup: func() History {
				return History{}
			},
			expected: nil,
		},
		{
			name: "single snapshot",
			setup: func() History {
				return History{
					{
						Digest:         "sha256:abc123",
						LastReconciled: metav1.NewTime(time.Now()),
					},
				}
			},
			expected: &Snapshot{
				Digest:         "sha256:abc123",
				LastReconciled: metav1.NewTime(time.Now()),
			},
		},
		{
			name: "multiple snapshots",
			setup: func() History {
				now := time.Now()
				return History{
					{
						Digest:         "sha256:new123",
						LastReconciled: metav1.NewTime(now),
					},
					{
						Digest:         "sha256:old123",
						LastReconciled: metav1.NewTime(now.Add(-1 * time.Hour)),
					},
				}
			},
			expected: &Snapshot{
				Digest:         "sha256:new123",
				LastReconciled: metav1.NewTime(time.Now()),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.setup()
			result := h.Latest()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("expected %v, got nil", tt.expected)
				} else if result.Digest != tt.expected.Digest {
					t.Errorf("expected digest %s, got %s", tt.expected.Digest, result.Digest)
				}
			}
		})
	}
}

func TestSnapshotString(t *testing.T) {
	now := time.Now()
	s := Snapshot{
		Digest:                 "sha256:abc123def456",
		LastReconciled:         metav1.NewTime(now),
		LastReconciledDuration: metav1.Duration{Duration: 45 * time.Second},
		LastReconciledStatus:   "Success",
	}

	result := s.String()

	// Should contain the short ID (first 8 chars after colon)
	if !strings.Contains(result, "id=abc123de") {
		t.Errorf("expected string to contain 'id=abc123de', got %s", result)
	}

	// Should contain status
	if !strings.Contains(result, "status=Success") {
		t.Errorf("expected string to contain 'status=Success', got %s", result)
	}

	// Should contain duration
	if !strings.Contains(result, "duration=45s") {
		t.Errorf("expected string to contain 'duration=45s', got %s", result)
	}
}
