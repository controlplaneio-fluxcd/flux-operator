// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr/funcr"
	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateAndApplyWorkloadIdentityConfig(t *testing.T) {
	tests := []struct {
		name                                           string
		version                                        string
		cluster                                        fluxcdv1.Cluster
		expectError                                    bool
		expectErrorMsg                                 string
		expectRemovePermission                         bool
		expectAddPermission                            bool
		expectPatchContains                            string
		expectPatchNotContains                         string
		expectMultitenantWorkloadIdentityPatchApplied  bool
		expectObjectLevelLegacyWarning                 bool
		expectMultitenantWorkloadIdentityLegacyWarning bool
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
		{
			name:           "pre-2.6.0 with multitenantObjectIdentity returns error",
			version:        "2.5.0",
			cluster:        fluxcdv1.Cluster{MultitenantObjectIdentity: true},
			expectError:    true,
			expectErrorMsg: "not supported in Flux versions < 2.6.0",
		},
		{
			name:           "pre-2.6.0 with objectIdentityRBAC returns error",
			version:        "2.5.0",
			cluster:        fluxcdv1.Cluster{ObjectIdentityRBAC: true},
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
		{
			name:           "2.6.x with multitenantObjectIdentity returns error",
			version:        "2.6.0",
			cluster:        fluxcdv1.Cluster{MultitenantObjectIdentity: true},
			expectError:    true,
			expectErrorMsg: "not supported in Flux versions 2.6.x",
		},
		{
			name:                "2.6.x with objectIdentityRBAC adds feature gate patch",
			version:             "2.6.5",
			cluster:             fluxcdv1.Cluster{ObjectIdentityRBAC: true},
			expectPatchContains: "ObjectLevelWorkloadIdentity=true",
		},
		// Flux >= 2.7.0 and < 2.10.0 tests.
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
			expectErrorMsg: "objectIdentityRBAC must be set to true",
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
		{
			name:                "2.7.0 with objectIdentityRBAC adds feature gate for all controllers",
			version:             "2.7.0",
			cluster:             fluxcdv1.Cluster{ObjectIdentityRBAC: true},
			expectPatchContains: "helm-controller",
		},
		{
			name:    "2.7.0 objectIdentityRBAC=true multitenantObjectIdentity=true adds multitenant patch",
			version: "2.7.0",
			cluster: fluxcdv1.Cluster{
				ObjectIdentityRBAC:        true,
				MultitenantObjectIdentity: true,
			},
			expectMultitenantWorkloadIdentityPatchApplied: true,
		},
		// Flux >= 2.10.0 tests.
		{
			name:                   "2.10.0 with defaults does not patch feature gate or permissions",
			version:                "2.10.0",
			expectPatchNotContains: "ObjectLevelWorkloadIdentity",
		},
		{
			name:                           "2.10.0 objectLevel=true adds SA token permission without feature gate and logs warning",
			version:                        "2.10.0",
			cluster:                        fluxcdv1.Cluster{ObjectLevelWorkloadIdentity: true},
			expectAddPermission:            true,
			expectPatchNotContains:         "ObjectLevelWorkloadIdentity",
			expectObjectLevelLegacyWarning: true,
		},
		{
			name:                   "2.10.0 objectIdentityRBAC=true adds SA token permission without warning",
			version:                "2.10.0",
			cluster:                fluxcdv1.Cluster{ObjectIdentityRBAC: true},
			expectAddPermission:    true,
			expectPatchNotContains: "ObjectLevelWorkloadIdentity",
		},
		{
			name:           "2.10.0 objectLevel=false multitenant=true returns error",
			version:        "2.10.0",
			cluster:        fluxcdv1.Cluster{MultitenantWorkloadIdentity: true},
			expectError:    true,
			expectErrorMsg: "objectIdentityRBAC must be set to true",
		},
		{
			name:    "2.10.0 objectLevel=true multitenant=true adds permission and multitenant patch",
			version: "2.10.0",
			cluster: fluxcdv1.Cluster{
				ObjectLevelWorkloadIdentity: true,
				MultitenantWorkloadIdentity: true,
			},
			expectAddPermission:                            true,
			expectPatchNotContains:                         "ObjectLevelWorkloadIdentity",
			expectMultitenantWorkloadIdentityPatchApplied:  true,
			expectObjectLevelLegacyWarning:                 true,
			expectMultitenantWorkloadIdentityLegacyWarning: true,
		},
		{
			name:    "2.10.0 objectIdentityRBAC=true multitenant=true adds permission and multitenant patch",
			version: "2.10.0",
			cluster: fluxcdv1.Cluster{
				ObjectIdentityRBAC:          true,
				MultitenantWorkloadIdentity: true,
			},
			expectAddPermission:                            true,
			expectPatchNotContains:                         "ObjectLevelWorkloadIdentity",
			expectMultitenantWorkloadIdentityPatchApplied:  true,
			expectMultitenantWorkloadIdentityLegacyWarning: true,
		},
		{
			name:    "2.10.0 objectIdentityRBAC=true multitenantObjectIdentity=true adds permission and multitenant patch",
			version: "2.10.0",
			cluster: fluxcdv1.Cluster{
				ObjectIdentityRBAC:        true,
				MultitenantObjectIdentity: true,
			},
			expectAddPermission:                           true,
			expectPatchNotContains:                        "ObjectLevelWorkloadIdentity",
			expectMultitenantWorkloadIdentityPatchApplied: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Reset the package-level once guards so each case can assert
			// whether the legacy warnings are emitted.
			objectLevelWorkloadIdentityOnce = sync.Once{}
			multitenantWorkloadIdentityOnce = sync.Once{}

			var logBuffer bytes.Buffer
			logger := funcr.New(func(_, args string) {
				logBuffer.WriteString(args)
			}, funcr.Options{})

			opts := MakeDefaultOptions()
			opts.Version = tt.version
			opts.Logger = logger

			err := opts.ValidateAndApplyWorkloadIdentityConfig(tt.cluster)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.expectErrorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(opts.RemovePermissionForCreatingServiceAccountTokens).To(Equal(tt.expectRemovePermission))
			g.Expect(opts.AddPermissionForCreatingServiceAccountTokens).To(Equal(tt.expectAddPermission))

			if tt.expectPatchContains != "" {
				g.Expect(opts.Patches).To(ContainSubstring(tt.expectPatchContains))
			}

			if tt.expectPatchNotContains != "" {
				g.Expect(opts.Patches).NotTo(ContainSubstring(tt.expectPatchNotContains))
			}

			if tt.expectMultitenantWorkloadIdentityPatchApplied {
				g.Expect(opts.Patches).To(ContainSubstring("default-service-account"))
			}

			if tt.expectObjectLevelLegacyWarning {
				g.Expect(logBuffer.String()).To(ContainSubstring(objectLevelWorkloadIdentityLegacyWarning))
			} else {
				g.Expect(logBuffer.String()).NotTo(ContainSubstring(objectLevelWorkloadIdentityLegacyWarning))
			}

			if tt.expectMultitenantWorkloadIdentityLegacyWarning {
				g.Expect(logBuffer.String()).To(ContainSubstring(multitenantWorkloadIdentityLegacyWarning))
			} else {
				g.Expect(logBuffer.String()).NotTo(ContainSubstring(multitenantWorkloadIdentityLegacyWarning))
			}
		})
	}
}

func TestValidateAndApplyWorkloadIdentityConfig_LegacyWarningsLoggedOnce(t *testing.T) {
	g := NewWithT(t)

	objectLevelWorkloadIdentityOnce = sync.Once{}
	multitenantWorkloadIdentityOnce = sync.Once{}

	var logBuffer bytes.Buffer
	logger := funcr.New(func(_, args string) {
		logBuffer.WriteString(args)
	}, funcr.Options{})

	opts := MakeDefaultOptions()
	opts.Version = "2.10.0"
	opts.Logger = logger

	cluster := fluxcdv1.Cluster{
		ObjectLevelWorkloadIdentity: true,
		MultitenantWorkloadIdentity: true,
	}

	g.Expect(opts.ValidateAndApplyWorkloadIdentityConfig(cluster)).To(Succeed())
	g.Expect(opts.ValidateAndApplyWorkloadIdentityConfig(cluster)).To(Succeed())

	g.Expect(strings.Count(logBuffer.String(), objectLevelWorkloadIdentityLegacyWarning)).To(Equal(1))
	g.Expect(strings.Count(logBuffer.String(), multitenantWorkloadIdentityLegacyWarning)).To(Equal(1))
}
