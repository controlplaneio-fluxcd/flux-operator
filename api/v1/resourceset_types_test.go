// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceSetGetInterval(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    time.Duration
	}{
		{
			name:     "default interval when no annotation",
			expected: 60 * time.Minute,
		},
		{
			name:        "custom interval from annotation",
			annotations: map[string]string{ReconcileEveryAnnotation: "10m"},
			expected:    10 * time.Minute,
		},
		{
			name:        "default interval on invalid duration",
			annotations: map[string]string{ReconcileEveryAnnotation: "bad"},
			expected:    60 * time.Minute,
		},
		{
			name:        "zero when disabled",
			annotations: map[string]string{ReconcileAnnotation: "disabled"},
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			rs := &ResourceSet{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(rs.GetInterval()).To(Equal(tt.expected))
		})
	}
}

func TestResourceSetGetTimeout(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    time.Duration
	}{
		{
			name:     "default timeout when no annotation",
			expected: 5 * time.Minute,
		},
		{
			name:        "custom timeout from annotation",
			annotations: map[string]string{ReconcileTimeoutAnnotation: "15m"},
			expected:    15 * time.Minute,
		},
		{
			name:        "default timeout on invalid duration",
			annotations: map[string]string{ReconcileTimeoutAnnotation: "invalid"},
			expected:    5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			rs := &ResourceSet{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(rs.GetTimeout()).To(Equal(tt.expected))
		})
	}
}

func TestResourceSetIsDisabled(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:     "not disabled when no annotation",
			expected: false,
		},
		{
			name:        "disabled when annotation set",
			annotations: map[string]string{ReconcileAnnotation: "disabled"},
			expected:    true,
		},
		{
			name:        "not disabled for other values",
			annotations: map[string]string{ReconcileAnnotation: "enabled"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			rs := &ResourceSet{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(rs.IsDisabled()).To(Equal(tt.expected))
		})
	}
}

func TestResourceSetGetInputStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy *InputStrategySpec
		expected string
	}{
		{
			name:     "defaults to Flatten when nil",
			strategy: nil,
			expected: InputStrategyFlatten,
		},
		{
			name:     "defaults to Flatten when empty name",
			strategy: &InputStrategySpec{},
			expected: InputStrategyFlatten,
		},
		{
			name:     "returns specified strategy",
			strategy: &InputStrategySpec{Name: InputStrategyPermute},
			expected: InputStrategyPermute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			rs := &ResourceSet{}
			rs.Spec.InputStrategy = tt.strategy
			g.Expect(rs.GetInputStrategy()).To(Equal(tt.expected))
		})
	}
}

func TestNewResourceSetInput(t *testing.T) {
	t.Run("merges defaults for missing keys", func(t *testing.T) {
		g := NewWithT(t)
		defaults := map[string]any{"env": "staging", "region": "us-east-1"}
		values := map[string]any{"env": "production"}

		input, err := NewResourceSetInput(defaults, values)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(input).To(HaveLen(2))
		g.Expect(input).To(HaveKey("env"))
		g.Expect(input).To(HaveKey("region"))
		// env should be "production" (not overwritten by default)
		g.Expect(string(input["env"].Raw)).To(Equal(`"production"`))
		g.Expect(string(input["region"].Raw)).To(Equal(`"us-east-1"`))
	})

	t.Run("handles empty defaults", func(t *testing.T) {
		g := NewWithT(t)
		values := map[string]any{"key": "value"}

		input, err := NewResourceSetInput(nil, values)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(input).To(HaveLen(1))
	})

	t.Run("handles numeric values", func(t *testing.T) {
		g := NewWithT(t)
		values := map[string]any{"replicas": 3}

		input, err := NewResourceSetInput(nil, values)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(input["replicas"].Raw)).To(Equal("3"))
	})
}
