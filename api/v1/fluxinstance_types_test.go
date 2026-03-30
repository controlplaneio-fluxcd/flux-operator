// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFluxInstanceGetInterval(t *testing.T) {
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
			annotations: map[string]string{ReconcileEveryAnnotation: "5m"},
			expected:    5 * time.Minute,
		},
		{
			name:        "default interval on invalid duration",
			annotations: map[string]string{ReconcileEveryAnnotation: "not-a-duration"},
			expected:    60 * time.Minute,
		},
		{
			name:        "zero when disabled",
			annotations: map[string]string{ReconcileAnnotation: "disabled"},
			expected:    0,
		},
		{
			name:        "zero when disabled case-insensitive",
			annotations: map[string]string{ReconcileAnnotation: "Disabled"},
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			instance := &FluxInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(instance.GetInterval()).To(Equal(tt.expected))
		})
	}
}

func TestFluxInstanceGetArtifactInterval(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    time.Duration
	}{
		{
			name:     "default interval when no annotation",
			expected: 10 * time.Minute,
		},
		{
			name:        "custom interval from annotation",
			annotations: map[string]string{ReconcileArtifactEveryAnnotation: "30s"},
			expected:    30 * time.Second,
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
			instance := &FluxInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(instance.GetArtifactInterval()).To(Equal(tt.expected))
		})
	}
}

func TestFluxInstanceGetTimeout(t *testing.T) {
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
			annotations: map[string]string{ReconcileTimeoutAnnotation: "10m"},
			expected:    10 * time.Minute,
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
			instance := &FluxInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(instance.GetTimeout()).To(Equal(tt.expected))
		})
	}
}

func TestFluxInstanceIsDisabled(t *testing.T) {
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
			name:        "disabled case-insensitive",
			annotations: map[string]string{ReconcileAnnotation: "DISABLED"},
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
			instance := &FluxInstance{
				ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations},
			}
			g.Expect(instance.IsDisabled()).To(Equal(tt.expected))
		})
	}
}

func TestFluxInstanceGetComponents(t *testing.T) {
	tests := []struct {
		name       string
		components []Component
		expected   []string
	}{
		{
			name: "defaults to core controllers when empty",
			expected: []string{
				FluxSourceController,
				FluxKustomizeController,
				FluxHelmController,
				FluxNotificationController,
			},
		},
		{
			name:       "returns specified components",
			components: []Component{"source-controller", "helm-controller"},
			expected:   []string{"source-controller", "helm-controller"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			instance := &FluxInstance{}
			instance.Spec.Components = tt.components
			g.Expect(instance.GetComponents()).To(Equal(tt.expected))
		})
	}
}

func TestFluxInstanceGetCluster(t *testing.T) {
	t.Run("defaults when nil", func(t *testing.T) {
		g := NewWithT(t)
		instance := &FluxInstance{}
		cluster := instance.GetCluster()
		g.Expect(cluster.Type).To(Equal("kubernetes"))
		g.Expect(cluster.Domain).To(Equal("cluster.local"))
		g.Expect(cluster.NetworkPolicy).To(BeTrue())
	})

	t.Run("returns spec value when set", func(t *testing.T) {
		g := NewWithT(t)
		instance := &FluxInstance{}
		instance.Spec.Cluster = &Cluster{
			Type:          "openshift",
			Domain:        "custom.local",
			NetworkPolicy: false,
		}
		cluster := instance.GetCluster()
		g.Expect(cluster.Type).To(Equal("openshift"))
		g.Expect(cluster.Domain).To(Equal("custom.local"))
		g.Expect(cluster.NetworkPolicy).To(BeFalse())
	})
}
