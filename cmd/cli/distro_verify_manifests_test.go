// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

func TestDistroVerifyManifestsValidVerification(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	manifestDir, privateKeyFile, publicKeyFile, err := setupBasicVerifyTest(tempDir, "test.issuer")
	g.Expect(err).ToNot(HaveOccurred())

	// Create manifest file
	manifestContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`
	err = os.WriteFile(filepath.Join(manifestDir, "manifest.yaml"), []byte(manifestContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	signatureFile := filepath.Join(manifestDir, "signature.sig")

	// Sign the manifests
	_, err = executeCommand([]string{"distro", "sign", "manifests", manifestDir, "--key-set", privateKeyFile, "--attestation", signatureFile})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the manifests
	output, err := executeCommand([]string{"distro", "verify", "manifests", manifestDir, "--key-set", publicKeyFile, "--attestation", signatureFile})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("attestation issued by test.issuer"))
}

func TestDistroVerifyManifestsErrorCases(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "missing attestation flag",
			setupFunc: func(tempDir string) ([]string, error) {
				_, _, publicKeyFile, err := setupBasicVerifyTest(tempDir, "test.issuer")
				if err != nil {
					return nil, err
				}
				return []string{"distro", "verify", "manifests", tempDir, "--key-set", publicKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to read",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				keyDir := filepath.Join(tempDir, "keys")
				manifestDir := filepath.Join(tempDir, "manifests")
				if err := os.MkdirAll(keyDir, 0755); err != nil {
					return nil, err
				}
				if err := os.MkdirAll(manifestDir, 0755); err != nil {
					return nil, err
				}

				// Generate keys and create a valid signature first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", keyDir})
				if err != nil {
					return nil, err
				}

				// Create manifest file
				manifestContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`
				err = os.WriteFile(filepath.Join(manifestDir, "manifest.yaml"), []byte(manifestContent), 0644)
				if err != nil {
					return nil, err
				}

				// Find private key file
				privateKeyFile, _, err := findKeyFiles(keyDir)
				if err != nil {
					return nil, err
				}

				signatureFile := filepath.Join(manifestDir, "signature.sig")

				// Create a valid signature
				_, err = executeCommand([]string{"distro", "sign", "manifests", manifestDir, "--key-set", privateKeyFile, "--attestation", signatureFile})
				if err != nil {
					return nil, err
				}

				// Return verify command without key set
				return []string{"distro", "verify", "manifests", manifestDir, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "invalid signature file",
			setupFunc: func(tempDir string) ([]string, error) {
				keyDir := filepath.Join(tempDir, "keys")
				if err := os.MkdirAll(keyDir, 0755); err != nil {
					return nil, err
				}

				// Generate keys
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", keyDir})
				if err != nil {
					return nil, err
				}

				// Find public key file
				_, publicKeyFile, err := findKeyFiles(keyDir)
				if err != nil {
					return nil, err
				}

				// Create invalid signature file
				signatureFile := filepath.Join(tempDir, "invalid.sig")
				err = os.WriteFile(signatureFile, []byte("invalid.jwt.token"), 0644)
				if err != nil {
					return nil, err
				}

				return []string{"distro", "verify", "manifests", tempDir, "--key-set", publicKeyFile, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse",
		},
		{
			name: "mismatched checksum",
			setupFunc: func(tempDir string) ([]string, error) {
				keyDir := filepath.Join(tempDir, "keys")
				manifestDir := filepath.Join(tempDir, "manifests")
				if err := os.MkdirAll(keyDir, 0755); err != nil {
					return nil, err
				}
				if err := os.MkdirAll(manifestDir, 0755); err != nil {
					return nil, err
				}

				// Generate keys
				_, err := executeCommand([]string{"distro", "keygen", "sig", "mismatch.issuer", "--output-dir", keyDir})
				if err != nil {
					return nil, err
				}

				// Create initial manifest
				initialContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: initial-config
data:
  key: initial`
				err = os.WriteFile(filepath.Join(manifestDir, "manifest.yaml"), []byte(initialContent), 0644)
				if err != nil {
					return nil, err
				}

				// Find key files
				privateKeyFile, publicKeyFile, err := findKeyFiles(keyDir)
				if err != nil {
					return nil, err
				}

				signatureFile := filepath.Join(manifestDir, "signature.sig")

				// Sign with initial content
				_, err = executeCommand([]string{"distro", "sign", "manifests", manifestDir, "--key-set", privateKeyFile, "--attestation", signatureFile})
				if err != nil {
					return nil, err
				}

				// Modify content after signing to cause checksum mismatch
				modifiedContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: modified-config
data:
  key: modified`
				err = os.WriteFile(filepath.Join(manifestDir, "manifest.yaml"), []byte(modifiedContent), 0644)
				if err != nil {
					return nil, err
				}

				// Return verify command args (should fail due to checksum mismatch)
				return []string{"distro", "verify", "manifests", manifestDir, "--key-set", publicKeyFile, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "checksum mismatch",
		},
		{
			name: "invalid directory",
			setupFunc: func(tempDir string) ([]string, error) {
				keyDir := filepath.Join(tempDir, "keys")
				if err := os.MkdirAll(keyDir, 0755); err != nil {
					return nil, err
				}

				// Generate keys
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", keyDir})
				if err != nil {
					return nil, err
				}

				// Find public key file
				_, publicKeyFile, err := findKeyFiles(keyDir)
				if err != nil {
					return nil, err
				}

				signatureFile := filepath.Join(tempDir, "signature.sig")
				nonExistentDir := filepath.Join(tempDir, "nonexistent")

				return []string{"distro", "verify", "manifests", nonExistentDir, "--key-set", publicKeyFile, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create temporary directory for this test
			tempDir := t.TempDir()

			// Setup the test scenario
			args, err := tt.setupFunc(tempDir)
			g.Expect(err).ToNot(HaveOccurred())

			// Execute command
			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errorMessage != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorMessage))
				}
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(ContainSubstring("processing files:"))
			g.Expect(output).To(ContainSubstring("attestation issued by"))
			g.Expect(output).To(ContainSubstring("is valid"))
		})
	}
}

func TestDistroVerifyManifestsWithEnvVar(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()
	keyDir := filepath.Join(tempDir, "keys")
	manifestDir := filepath.Join(tempDir, "manifests")

	err := os.MkdirAll(keyDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(manifestDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Generate keys
	_, err = executeCommand([]string{"distro", "keygen", "sig", "env.verify.issuer", "--output-dir", keyDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find key files
	privateKeyFile, publicKeyFile, err := findKeyFiles(keyDir)
	g.Expect(err).ToNot(HaveOccurred())

	// Create manifest file
	manifestContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 2`
	err = os.WriteFile(filepath.Join(manifestDir, "deployment.yaml"), []byte(manifestContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	signatureFile := filepath.Join(manifestDir, "signature.sig")

	// Sign manifests
	_, err = executeCommand([]string{"distro", "sign", "manifests", manifestDir, "--key-set", privateKeyFile, "--attestation", signatureFile})
	g.Expect(err).ToNot(HaveOccurred())

	// Read the public key content
	publicKeyData, err := os.ReadFile(publicKeyFile)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable
	t.Setenv(distroSigPublicKeySetEnvVar, string(publicKeyData))

	// Verify using env var (no --key-set flag)
	args := []string{"distro", "verify", "manifests", manifestDir, "--attestation", signatureFile}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("processing files:"))
	g.Expect(output).To(ContainSubstring("deployment.yaml"))
	g.Expect(output).To(ContainSubstring("attestation issued by env.verify.issuer"))
}

func TestDistroVerifyManifestsKeyMatching(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()
	keyDir1 := filepath.Join(tempDir, "keys1")
	keyDir2 := filepath.Join(tempDir, "keys2")
	manifestDir := filepath.Join(tempDir, "manifests")

	err := os.MkdirAll(keyDir1, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(keyDir2, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	err = os.MkdirAll(manifestDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	// Generate two different key pairs
	_, err = executeCommand([]string{"distro", "keygen", "sig", "issuer1", "--output-dir", keyDir1})
	g.Expect(err).ToNot(HaveOccurred())
	_, err = executeCommand([]string{"distro", "keygen", "sig", "issuer2", "--output-dir", keyDir2})
	g.Expect(err).ToNot(HaveOccurred())

	// Find key pairs
	privateKeyFile1, publicKeyFile1, err := findKeyFiles(keyDir1)
	g.Expect(err).ToNot(HaveOccurred())
	_, publicKeyFile2, err := findKeyFiles(keyDir2)
	g.Expect(err).ToNot(HaveOccurred())

	// Create manifest
	manifestContent := `apiVersion: v1
kind: Service
metadata:
  name: test-service`
	err = os.WriteFile(filepath.Join(manifestDir, "service.yaml"), []byte(manifestContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	signatureFile := filepath.Join(manifestDir, "signature.sig")

	// Sign with first key pair
	_, err = executeCommand([]string{"distro", "sign", "manifests", manifestDir, "--key-set", privateKeyFile1, "--attestation", signatureFile})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify with correct public key (should succeed)
	output, err := executeCommand([]string{"distro", "verify", "manifests", manifestDir, "--key-set", publicKeyFile1, "--attestation", signatureFile})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("attestation issued by issuer1"))

	// Verify with wrong public key (should fail)
	_, err = executeCommand([]string{"distro", "verify", "manifests", manifestDir, "--key-set", publicKeyFile2, "--attestation", signatureFile})
	g.Expect(err).To(HaveOccurred())

	// The error should be about key ID not found, since the JWT contains key ID from first key pair
	// but we're trying to verify with second key pair which has different key IDs
	g.Expect(errors.Is(err, lkm.ErrKeyNotFound)).To(BeTrue())
}

// Helper function to setup basic key generation and manifest creation
func setupBasicVerifyTest(tempDir, issuer string) (manifestDir, privateKeyFile, publicKeyFile string, err error) {
	keyDir := filepath.Join(tempDir, "keys")
	manifestDir = filepath.Join(tempDir, "manifests")

	if err = os.MkdirAll(keyDir, 0755); err != nil {
		return
	}
	if err = os.MkdirAll(manifestDir, 0755); err != nil {
		return
	}

	// Generate keys
	_, err = executeCommand([]string{"distro", "keygen", "sig", issuer, "--output-dir", keyDir})
	if err != nil {
		return
	}

	privateKeyFile, publicKeyFile, err = findKeyFiles(keyDir)
	return
}
