// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestValidateAndPatchComponents(t *testing.T) {
	tests := []struct {
		name                string
		version             string
		components          []string
		expectError         bool
		expectErrorMsg      string
		expectPatchContains string
	}{
		{
			name:           "empty components returns error",
			version:        "2.7.0",
			components:     []string{},
			expectError:    true,
			expectErrorMsg: "no components defined",
		},
		{
			name:           "invalid component returns error",
			version:        "2.7.0",
			components:     []string{"invalid-controller"},
			expectError:    true,
			expectErrorMsg: "invalid component: invalid-controller",
		},
		{
			name:                "default components with notification-controller adds patch",
			version:             "2.7.0",
			components:          DefaultComponents,
			expectPatchContains: "notification.toolkit.fluxcd.io",
		},
		{
			name:       "components without notification-controller omits patch",
			version:    "2.7.0",
			components: []string{"source-controller", "kustomize-controller", "helm-controller"},
		},
		{
			name:           "source-watcher with version < 2.7.0 returns error",
			version:        "2.6.0",
			components:     []string{"source-controller", "kustomize-controller", "source-watcher"},
			expectError:    true,
			expectErrorMsg: "source-watcher is only supported in Flux versions >= 2.7.0",
		},
		{
			name:                "source-watcher with version >= 2.7.0 adds feature gate",
			version:             "2.7.0",
			components:          []string{"source-controller", "kustomize-controller", "source-watcher"},
			expectPatchContains: "ExternalArtifact=true",
		},
		{
			name:           "invalid version returns error",
			version:        "not-a-version",
			components:     DefaultComponents,
			expectError:    true,
			expectErrorMsg: "failed to parse Flux version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := MakeDefaultOptions()
			opts.Version = tt.version
			opts.Components = tt.components

			err := opts.ValidateAndPatchComponents()

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.expectErrorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			if tt.expectPatchContains != "" {
				g.Expect(opts.Patches).To(ContainSubstring(tt.expectPatchContains))
			}
		})
	}
}
