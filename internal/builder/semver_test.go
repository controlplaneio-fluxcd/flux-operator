// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestMatchVersion(t *testing.T) {
	fluxDir := filepath.Join("testdata", "flux")
	tests := []struct {
		name     string
		exp      string
		expected string
	}{
		{name: "exact", exp: "v2.3.0", expected: "v2.3.0"},
		{name: "patch", exp: "2.2.x", expected: "v2.2.1"},
		{name: "minor", exp: "2.x", expected: "v2.3.0"},
		{name: "latest", exp: "*", expected: "v2.3.0"},
		{name: "invalid", exp: "3.x", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ver, err := MatchVersion(fluxDir, tt.exp)
			if tt.expected != "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ver).To(Equal(tt.expected))
			} else {
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestExtractVersionDigest(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedVer    string
		expectedDigest string
		expectError    bool
	}{
		{
			name:           "proto with digest",
			input:          "oci://ghcr.io/org/app:v1.2.3@sha256:abc123",
			expectedVer:    "v1.2.3",
			expectedDigest: "sha256:abc123",
			expectError:    false,
		},
		{
			name:           "host with digest",
			input:          "localhost:5000/org/app:v1.2.3@sha256:def456",
			expectedVer:    "v1.2.3",
			expectedDigest: "sha256:def456",
			expectError:    false,
		},
		{
			name:           "version with digest only",
			input:          "v1.2.3-rc.1@sha256:789abc",
			expectedVer:    "v1.2.3-rc.1",
			expectedDigest: "sha256:789abc",
			expectError:    false,
		},
		{
			name:           "version only",
			input:          "v1.2.3",
			expectedVer:    "v1.2.3",
			expectedDigest: "",
			expectError:    false,
		},
		{
			name:           "proto without digest",
			input:          "oci://ghcr.io/org/app:v1.2.3",
			expectedVer:    "v1.2.3",
			expectedDigest: "",
			expectError:    false,
		},
		{
			name:           "host without digest",
			input:          "oci://localhost:5000/org/app:rc-abc123",
			expectedVer:    "rc-abc123",
			expectedDigest: "",
			expectError:    false,
		},
		{
			name:           "no version separator",
			input:          "v1.2.3@sha256:abc123",
			expectedVer:    "v1.2.3",
			expectedDigest: "sha256:abc123",
			expectError:    false,
		},
		{
			name:           "multiple @ symbols",
			input:          "v1.2.3@sha256:abc@def",
			expectedVer:    "",
			expectedDigest: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			version, digest, err := ExtractVersionDigest(tt.input)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(version).To(Equal(tt.expectedVer))
				g.Expect(digest).To(Equal(tt.expectedDigest))
			}
		})
	}
}
