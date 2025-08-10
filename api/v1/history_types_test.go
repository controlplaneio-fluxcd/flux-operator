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
		name           string
		setup          func() *History
		digest         string
		status         string
		expectedLength int
		expectedTotal  int64
	}{
		{
			name: "add new snapshot to empty history",
			setup: func() *History {
				h := &History{}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 1,
			expectedTotal:  1,
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
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 1,
			expectedTotal:  2,
		},
		{
			name: "update existing snapshot and move to front",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:xyz123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: 15 * time.Second},
						LastReconciledStatus:   "Failed",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 3,
			expectedTotal:  2,
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
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Failure",
			expectedLength: 2,
			expectedTotal:  1,
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
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 2,
			expectedTotal:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.setup()
			metadata := map[string]string{"test": "value"}

			h.Upsert(tt.digest, time.Now(), 45*time.Second, tt.status, metadata)

			if len(*h) != tt.expectedLength {
				t.Errorf("expectedLength length %d, got %d", tt.expectedLength, len(*h))
			}

			// Verify latest entry matches the upserted snapshot
			if h.Latest().Digest != tt.digest && h.Latest().LastReconciledStatus != tt.status {
				t.Errorf("expectedLength first snapshot digest %s status %s, got %s %s",
					tt.digest, tt.status, h.Latest().Digest, h.Latest().LastReconciledStatus)
			}

			// Verify total reconciliations
			if tt.expectedTotal != h.Latest().TotalReconciliations {
				t.Errorf("expectedLength total reconciliations to be %d, got %d",
					tt.expectedTotal, h.Latest().TotalReconciliations)
			}

			// Verify metadata was set
			for _, snapshot := range *h {
				if snapshot.Digest == tt.digest && snapshot.LastReconciledStatus == tt.status {
					if snapshot.Metadata["test"] != "value" {
						t.Errorf("expectedLength metadata 'test'='value', got %v", snapshot.Metadata)
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
		t.Errorf("expectedLength length %d after truncation, got %d", MaxHistorySize, len(*h))
	}

	// Verify it kept the most recent ones
	latest := (*h)[0]
	expectedLatestDigest := digests[6] // Last one added (index 6)
	if latest.Digest != expectedLatestDigest {
		t.Errorf("expectedLength latest digest %s, got %s", expectedLatestDigest, latest.Digest)
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
					t.Errorf("expectedLength nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("expectedLength %v, got nil", tt.expected)
				} else if result.Digest != tt.expected.Digest {
					t.Errorf("expectedLength digest %s, got %s", tt.expected.Digest, result.Digest)
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
		t.Errorf("expectedLength string to contain 'id=abc123de', got %s", result)
	}

	// Should contain status
	if !strings.Contains(result, "status=Success") {
		t.Errorf("expectedLength string to contain 'status=Success', got %s", result)
	}

	// Should contain duration
	if !strings.Contains(result, "duration=45s") {
		t.Errorf("expectedLength string to contain 'duration=45s', got %s", result)
	}
}
