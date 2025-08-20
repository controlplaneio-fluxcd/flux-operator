// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDistroVerifyArtifactsCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "successfully verifies artifacts with digest URLs",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the key files
				privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Define digest URLs for artifacts (no registry calls needed)
				digestURLs := []string{
					"ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150",
					"ghcr.io/fluxcd/source-controller@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					"oci://ghcr.io/fluxcd/kustomize-controller@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				}

				// First sign the artifacts to create an attestation
				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				signArgs := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile}
				for _, url := range digestURLs {
					signArgs = append(signArgs, "--url", url)
				}
				_, err = executeCommand(signArgs)
				if err != nil {
					return nil, err
				}

				// Now build the verify command args
				verifyArgs := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--attestation", attestationFile}
				for _, url := range digestURLs {
					verifyArgs = append(verifyArgs, "--url", url)
				}

				return verifyArgs, nil
			},
			expectError: false,
		},
		{
			name: "fails with missing attestation flag",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the public key file
				_, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				args := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--url", digestURL}

				return args, nil
			},
			expectError:  true,
			errorMessage: "--attestation flag is required",
		},
		{
			name: "fails with missing URL flags",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the public key file
				_, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				args := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--attestation", attestationFile}

				return args, nil
			},
			expectError:  true,
			errorMessage: "--url flag is required",
		},
		{
			name: "fails with missing key-set flag",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys and create a dummy attestation file first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Create a dummy attestation file
				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"

				// Sign to create the file
				_, err = executeCommand([]string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile, "--url", digestURL})
				if err != nil {
					return nil, err
				}

				// Now try to verify without key-set
				args := []string{"distro", "verify", "artifacts", "--attestation", attestationFile, "--url", digestURL}

				return args, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "fails with non-existent attestation file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the public key file
				_, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				nonExistentFile := filepath.Join(tempDir, "non-existent.jwt")
				args := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--attestation", nonExistentFile, "--url", digestURL}

				return args, nil
			},
			expectError:  true,
			errorMessage: "failed to read the attestation file",
		},
		{
			name: "fails when URL digest not found in attestation",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the key files
				privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Sign with one digest
				signedDigest := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				attestationFile := filepath.Join(tempDir, "artifacts.jwt")
				signArgs := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile, "--url", signedDigest}
				_, err = executeCommand(signArgs)
				if err != nil {
					return nil, err
				}

				// Try to verify with a different digest
				differentDigest := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
				verifyArgs := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--attestation", attestationFile, "--url", differentDigest}

				return verifyArgs, nil
			},
			expectError:  true,
			errorMessage: "verification failed",
		},
		{
			name: "successfully verifies single artifact",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test-issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the key files
				privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Sign and verify the same digest
				digestURL := "ghcr.io/controlplaneio-fluxcd/flux-operator@sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150"
				attestationFile := filepath.Join(tempDir, "single-artifact.jwt")

				// First sign
				signArgs := []string{"distro", "sign", "artifacts", "--key-set", privateKeyFile, "--attestation", attestationFile, "--url", digestURL}
				_, err = executeCommand(signArgs)
				if err != nil {
					return nil, err
				}

				// Then verify
				verifyArgs := []string{"distro", "verify", "artifacts", "--key-set", publicKeyFile, "--attestation", attestationFile, "--url", digestURL}
				return verifyArgs, nil
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
				g.Expect(output).To(ContainSubstring("verified"))

				// Verify the output shows digests were extracted from URLs (not fetched from registry)
				g.Expect(output).To(ContainSubstring("(from URL)"))
				g.Expect(output).ToNot(ContainSubstring("(from registry)"))

				// Each URL should appear in the output with a checkmark
				for _, arg := range args {
					if strings.Contains(arg, "@sha256:") {
						digest := strings.Split(arg, "@")[1] // Get the sha256:... part
						g.Expect(output).To(ContainSubstring("✔ " + digest))
					}
				}
			}
		})
	}
}
