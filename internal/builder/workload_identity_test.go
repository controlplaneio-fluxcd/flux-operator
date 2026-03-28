// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateAndApplyWorkloadIdentityConfig(t *testing.T) {
	tests := []struct {
		name                                          string
		version                                       string
		cluster                                       fluxcdv1.Cluster
		expectError                                   bool
		expectErrorMsg                                string
		expectRemovePermission                        bool
		expectPatchContains                           string
		expectPatchNotContains                        string
		expectMultitenantWorkloadIdentityPatchApplied bool
	}{
		// Invalid version.
		{
			name:        "invalid version returns error",
			version:     "not-a-version",
			expectError: true,
		},
		// Flux < 2.6.0 tests.
		{
			name:    "pre-2.6.0 with defaults succeeds with no patches",
			version: "2.5.0",
		},
		{
			name:           "pre-2.6.0 with objectLevelWorkloadIdentity returns error",
			version:        "2.5.0",
			cluster:        fluxcdv1.Cluster{ObjectLevelWorkloadIdentity: true},
			expectError:    true,
			expectErrorMsg: "not supported in Flux versions < 2.6.0",
		},
		{
			name:           "pre-2.6.0 with multitenantWorkloadIdentity returns error",
			version:        "2.5.0",
			cluster:        fluxcdv1.Cluster{MultitenantWorkloadIdentity: true},
			expectError:    true,
			expectErrorMsg: "not supported in Flux versions < 2.6.0",
		},
		// Flux 2.6.x tests.
		{
			name:                   "2.6.x with defaults removes SA token permission",
			version:                "2.6.0",
			expectRemovePermission: true,
		},
		{
			name:                "2.6.x with objectLevelWorkloadIdentity adds feature gate patch",
			version:             "2.6.5",
			cluster:             fluxcdv1.Cluster{ObjectLevelWorkloadIdentity: true},
			expectPatchContains: "ObjectLevelWorkloadIdentity=true",
		},
		{
			name:           "2.6.x with multitenantWorkloadIdentity returns error",
			version:        "2.6.0",
			cluster:        fluxcdv1.Cluster{MultitenantWorkloadIdentity: true},
			expectError:    true,
			expectErrorMsg: "not supported in Flux versions 2.6.x",
		},
		// Flux >= 2.7.0 tests.
		{
			name:                   "2.7.0 with defaults disables feature gate and removes SA token permission",
			version:                "2.7.0",
			expectPatchContains:    "ObjectLevelWorkloadIdentity=false",
			expectRemovePermission: true,
		},
		{
			name:           "2.7.0 objectLevel=false multitenant=true returns error",
			version:        "2.7.0",
			cluster:        fluxcdv1.Cluster{MultitenantWorkloadIdentity: true},
			expectError:    true,
			expectErrorMsg: "objectLevelWorkloadIdentity must be set to true",
		},
		{
			name:                "2.7.0 objectLevel=true adds feature gate for all controllers",
			version:             "2.7.0",
			cluster:             fluxcdv1.Cluster{ObjectLevelWorkloadIdentity: true},
			expectPatchContains: "helm-controller",
		},
		{
			name:    "2.7.0 objectLevel=true multitenant=true adds multitenant patch",
			version: "2.7.0",
			cluster: fluxcdv1.Cluster{
				ObjectLevelWorkloadIdentity: true,
				MultitenantWorkloadIdentity: true,
			},
			expectMultitenantWorkloadIdentityPatchApplied: true,
		},
		{
			name:    "2.7.0 multitenant with custom service accounts",
			version: "2.7.0",
			cluster: fluxcdv1.Cluster{
				ObjectLevelWorkloadIdentity: true,
				MultitenantWorkloadIdentity: true,
				TenantDefaultServiceAccount: "custom-sa",
			},
			expectPatchContains: "custom-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := MakeDefaultOptions()
			opts.Version = tt.version

			err := opts.ValidateAndApplyWorkloadIdentityConfig(tt.cluster)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.expectErrorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			if tt.expectRemovePermission {
				g.Expect(opts.RemovePermissionForCreatingServiceAccountTokens).To(BeTrue())
			}

			if tt.expectPatchContains != "" {
				g.Expect(opts.Patches).To(ContainSubstring(tt.expectPatchContains))
			}

			if tt.expectPatchNotContains != "" {
				g.Expect(opts.Patches).NotTo(ContainSubstring(tt.expectPatchNotContains))
			}

			if tt.expectMultitenantWorkloadIdentityPatchApplied {
				g.Expect(opts.Patches).To(ContainSubstring("default-service-account"))
			}
		})
	}
}
