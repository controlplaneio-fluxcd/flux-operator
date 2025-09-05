// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

func TestDistroEncryptTokenCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "encrypt with key set file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				keySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "input.txt")
				err = os.WriteFile(inputPath, []byte("secret data"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")

				return []string{"distro", "encrypt", "token", "--key-set", keySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "encrypt with specific key ID",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				keySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "input.txt")
				err = os.WriteFile(inputPath, []byte("secret data"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")
				keyID := publicKeySet.Keys[0].KeyID

				return []string{"distro", "encrypt", "token", "--key-set", keySetPath, "--key-id", keyID, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				inputPath := filepath.Join(tempDir, "input.txt")
				err := os.WriteFile(inputPath, []byte("secret data"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")

				return []string{"distro", "encrypt", "token", "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "invalid key set file",
			setupFunc: func(tempDir string) ([]string, error) {
				keySetPath := filepath.Join(tempDir, "invalid.jwks")
				err := os.WriteFile(keySetPath, []byte("invalid json"), 0644)
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "input.txt")
				err = os.WriteFile(inputPath, []byte("secret data"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")

				return []string{"distro", "encrypt", "token", "--key-set", keySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse public key set",
		},
		{
			name: "missing input file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				keySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")
				nonexistentInput := filepath.Join(tempDir, "nonexistent.txt")

				return []string{"distro", "encrypt", "token", "--key-set", keySetPath, "--input", nonexistentInput, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to read input",
		},
		{
			name: "invalid key ID",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				keySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "input.txt")
				err = os.WriteFile(inputPath, []byte("secret data"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.jwe")

				return []string{"distro", "encrypt", "token", "--key-set", keySetPath, "--key-id", "invalid-key-id", "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to encrypt data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create temporary directory for this test
			tempDir := t.TempDir()

			// Setup the test case
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

			// Check if output file was created and contains JWE token
			var outputPath string
			for i, arg := range args {
				if arg == "--output" && i+1 < len(args) {
					outputPath = args[i+1]
					break
				}
			}

			if outputPath != "" {
				g.Expect(output).To(ContainSubstring("encrypted data written to"))

				// Verify the output file exists and contains a JWE token
				jweData, err := os.ReadFile(outputPath)
				g.Expect(err).ToNot(HaveOccurred())

				// JWE tokens have 5 parts separated by dots
				jweString := string(jweData)
				parts := strings.Split(jweString, ".")
				g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")
				g.Expect(jweString).ToNot(BeEmpty())
			}
		})
	}
}

func TestDistroEncryptTokenCmdWithEnvVar(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, _, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	keySetPath := filepath.Join(tempDir, "public.jwks")
	err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Read key set content
	keySetData, err := os.ReadFile(keySetPath)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable using t.Setenv for automatic cleanup
	t.Setenv(distroEncPublicKeySetEnvVar, string(keySetData))

	inputPath := filepath.Join(tempDir, "input.txt")
	err = os.WriteFile(inputPath, []byte("secret data from env"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	outputPath := filepath.Join(tempDir, "output.jwe")

	// Execute command without --key-set flag (should use env var)
	args := []string{"distro", "encrypt", "token", "--input", inputPath, "--output", outputPath}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("encrypted data written to"))

	// Verify the output file exists and contains a JWE token
	jweData, err := os.ReadFile(outputPath)
	g.Expect(err).ToNot(HaveOccurred())

	// JWE tokens have 5 parts separated by dots
	jweString := string(jweData)
	parts := strings.Split(jweString, ".")
	g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")
	g.Expect(jweString).ToNot(BeEmpty())
}

func TestDistroEncryptTokenCmdRoundTrip(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	publicKeySetPath := filepath.Join(tempDir, "public.jwks")
	err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Create test data
	originalData := "This is secret data that should be encrypted and decrypted correctly!"
	inputPath := filepath.Join(tempDir, "input.txt")
	err = os.WriteFile(inputPath, []byte(originalData), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	outputPath := filepath.Join(tempDir, "encrypted.jwe")

	// Encrypt the data
	args := []string{"distro", "encrypt", "token", "--key-set", publicKeySetPath, "--input", inputPath, "--output", outputPath}
	_, err = executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the output file contains valid JWE
	jweData, err := os.ReadFile(outputPath)
	g.Expect(err).ToNot(HaveOccurred())

	// Decrypt using the lkm package to verify round-trip
	decryptedData, err := lkm.DecryptTokenWithKeySet(jweData, privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(decryptedData)).To(Equal(originalData))
}
