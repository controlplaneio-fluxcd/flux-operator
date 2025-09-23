// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDistroSignArtifactsCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "successfully signs artifacts with digest URLs",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Define digest URLs for artifacts (no registry calls needed)
				digestURLs := []string{
					"ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150",
					"ghcr.io/fluxcd/source-controller@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					"oci://ghcr.io/fluxcd/kustomize-controller@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				}

				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				args := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile}
				for _, url := range digestURLs {
					args = append(args, "--url", url)
				}

				return args, nil
			},
			expectError: false,
		},
		{
			name: "fails with missing attestation flag",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				args := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--url", digestURL}

				return args, nil
			},
			expectError:  true,
			errorMessage: "--attestation flag is required",
		},
		{
			name: "fails with missing URL flags",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				args := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile}

				return args, nil
			},
			expectError:  true,
			errorMessage: "--url flag is required",
		},
		{
			name: "fails with missing key-set flag",
			setupFunc: func(tempDir string) ([]string, error) {
				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				args := []string{"distro", "sign", "artifacts", "--attestation", attestationFile, "--url", digestURL}

				return args, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "successfully signs single artifact",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				attestationFile := filepath.Join(tempDir, "single-artifact.jwt")
				args := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile, "--url", digestURL}

				return args, nil
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tempDir := t.TempDir()

			args, err := tt.setupFunc(tempDir)
			g.Expect(err).ToNot(HaveOccurred())

			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errorMessage != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorMessage))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(ContainSubstring("processing artifacts:"))
				g.Expect(output).To(ContainSubstring("✔ attestation written to:"))

				// Verify the attestation file was created
				attestationFile := ""
				for i, arg := range args {
					if arg == "--attestation" && i+1 < len(args) {
						attestationFile = args[i+1]
						break
					}
				}
				g.Expect(attestationFile).ToNot(BeEmpty())

				// Check that the attestation file exists and is not empty
				info, err := os.Stat(attestationFile)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(info.Size()).To(BeNumerically(">", 0))

				// Count URLs processed
				urlCount := 0
				for _, arg := range args {
					if strings.Contains(arg, "@sha256:") {
						urlCount++
					}
				}
				g.Expect(output).To(ContainSubstring("processing artifacts:"))
				// Each URL should appear in the output
				for _, arg := range args {
					if strings.Contains(arg, "@sha256:") {
						baseURL := strings.Split(arg, "@")[0]
						g.Expect(output).To(ContainSubstring(baseURL))
					}
				}
			}
		})
	}
}

func TestHasArtifactDigest(t *testing.T) {
	dig := "sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
	t.Run("extracts digest from valid OCI URL with sha256", func(t *testing.T) {
		g := NewWithT(t)
		// Use a real SHA256 digest (64 hex characters)
		url := "oci://ghcr.io/controlplaneio-fluxcd/flux-operator@" + dig

		digest, err := hasArtifactDigest(url)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(digest).To(Equal(dig))
	})

	t.Run("fails with tag-based URL", func(t *testing.T) {
		g := NewWithT(t)
		url := "oci://ghcr.io/controlplaneio-fluxcd/flux-operator:latest"

		_, err := hasArtifactDigest(url)

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails with invalid OCI URL", func(t *testing.T) {
		g := NewWithT(t)
		url := "invalid-url"

		_, err := hasArtifactDigest(url)

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails with empty URL", func(t *testing.T) {
		g := NewWithT(t)
		url := ""

		_, err := hasArtifactDigest(url)

		g.Expect(err).To(HaveOccurred())
	})

	t.Run("handles URL without oci:// prefix", func(t *testing.T) {
		g := NewWithT(t)
		url := "ghcr.io/controlplaneio-fluxcd/flux-operator@" + dig

		digest, err := hasArtifactDigest(url)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(digest).To(Equal(dig))
	})
}
