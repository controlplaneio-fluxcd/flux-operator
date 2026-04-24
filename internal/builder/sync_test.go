// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"testing"

	"github.com/fluxcd/pkg/auth/aws"
	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateSync(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		sync           *Sync
		expectError    bool
		expectErrorMsg string
	}{
		{
			name:    "no sync is valid",
			version: "2.8.0",
			sync:    nil,
		},
		{
			name:    "GitRepository with github provider is valid on older versions",
			version: "2.6.0",
			sync:    &Sync{Kind: fluxcdv1.FluxGitRepositoryKind, Provider: "github"},
		},
		{
			name:    "GitRepository with aws provider is valid on 2.9.0",
			version: "2.9.0",
			sync:    &Sync{Kind: fluxcdv1.FluxGitRepositoryKind, Provider: aws.ProviderName},
		},
		{
			name:    "GitRepository with aws provider is valid on 2.10.0",
			version: "2.10.0",
			sync:    &Sync{Kind: fluxcdv1.FluxGitRepositoryKind, Provider: aws.ProviderName},
		},
		{
			name:           "GitRepository with aws provider is rejected on 2.8.0",
			version:        "2.8.0",
			sync:           &Sync{Kind: fluxcdv1.FluxGitRepositoryKind, Provider: aws.ProviderName},
			expectError:    true,
			expectErrorMsg: "requires Flux version >= 2.9.0",
		},
		{
			name:    "OCIRepository with aws provider is valid on older versions",
			version: "2.6.0",
			sync:    &Sync{Kind: fluxcdv1.FluxOCIRepositoryKind, Provider: aws.ProviderName},
		},
		{
			name:    "Bucket with aws provider is valid on older versions",
			version: "2.6.0",
			sync:    &Sync{Kind: fluxcdv1.FluxBucketKind, Provider: aws.ProviderName},
		},
		{
			name:           "invalid version returns error when validating aws GitRepository",
			version:        "not-a-version",
			sync:           &Sync{Kind: fluxcdv1.FluxGitRepositoryKind, Provider: aws.ProviderName},
			expectError:    true,
			expectErrorMsg: "failed to parse Flux version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := MakeDefaultOptions()
			opts.Version = tt.version
			opts.Sync = tt.sync

			err := opts.ValidateSync()

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.expectErrorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}
