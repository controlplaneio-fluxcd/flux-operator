// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetProfileMultitenant(t *testing.T) {
	tests := []struct {
		name                 string
		defaultSA            string
		version              string
		expectControllers    string
		expectServiceAccount string
	}{
		{
			name:                 "before 2.10 excludes source-watcher from impersonation",
			version:              "v2.9.0",
			expectControllers:    "kustomize-controller|helm-controller",
			expectServiceAccount: defaultServiceAccount,
		},
		{
			name:                 "2.10 includes source-watcher in impersonation",
			version:              "v2.10.0",
			expectControllers:    "kustomize-controller|helm-controller|source-watcher",
			expectServiceAccount: defaultServiceAccount,
		},
		{
			name:                 "custom default service account",
			defaultSA:            "tenant-sa",
			version:              "v2.11.0",
			expectControllers:    "kustomize-controller|helm-controller|source-watcher",
			expectServiceAccount: "tenant-sa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			profile := GetProfileMultitenant(tt.defaultSA, tt.version)

			g.Expect(profile).To(ContainSubstring(fmt.Sprintf(`name: "(%s)"`, tt.expectControllers)))
			g.Expect(profile).To(ContainSubstring(fmt.Sprintf("--default-service-account=%s", tt.expectServiceAccount)))
			g.Expect(profile).To(ContainSubstring("--no-cross-namespace-refs=true"))
		})
	}
}
