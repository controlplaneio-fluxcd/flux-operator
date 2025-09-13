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

func TestDistroDecryptTokenCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "decrypt with key set file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create test data and encrypt it
				originalData := "secret test data"
				jweToken, err := lkm.EncryptTokenWithKeySet([]byte(originalData), publicKeySet, "")
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "encrypted.jwe")
				err = os.WriteFile(inputPath, []byte(jweToken), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "decrypted.txt")

				return []string{"distro", "decrypt", "token", "--key-set", privateKeySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				// Create encrypted data (this will fail during execution)
				inputPath := filepath.Join(tempDir, "encrypted.jwe")
				err := os.WriteFile(inputPath, []byte("fake.jwe.token.data.here"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.txt")

				return []string{"distro", "decrypt", "token", "--input", inputPath, "--output", outputPath}, nil
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

				inputPath := filepath.Join(tempDir, "encrypted.jwe")
				err = os.WriteFile(inputPath, []byte("fake.jwe.token.data.here"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.txt")

				return []string{"distro", "decrypt", "token", "--key-set", keySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse private key set",
		},
		{
			name: "missing input file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				_, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.txt")
				nonexistentInput := filepath.Join(tempDir, "nonexistent.jwe")

				return []string{"distro", "decrypt", "token", "--key-set", privateKeySetPath, "--input", nonexistentInput, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to read input JWE",
		},
		{
			name: "invalid JWE token",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				_, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create invalid JWE data
				inputPath := filepath.Join(tempDir, "invalid.jwe")
				err = os.WriteFile(inputPath, []byte("invalid.jwe.token"), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.txt")

				return []string{"distro", "decrypt", "token", "--key-set", privateKeySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to decrypt data",
		},
		{
			name: "wrong private key",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate first key pair for encryption
				publicKeySet1, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				// Generate second key pair for decryption (different keys)
				_, privateKeySet2, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet2)
				if err != nil {
					return nil, err
				}

				// Encrypt with first key pair
				originalData := "secret test data"
				jweToken, err := lkm.EncryptTokenWithKeySet([]byte(originalData), publicKeySet1, "")
				if err != nil {
					return nil, err
				}

				inputPath := filepath.Join(tempDir, "encrypted.jwe")
				err = os.WriteFile(inputPath, []byte(jweToken), 0644)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.txt")

				return []string{"distro", "decrypt", "token", "--key-set", privateKeySetPath, "--input", inputPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to decrypt data",
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

			// Check if output file was created and contains the decrypted data
			var outputPath string
			for i, arg := range args {
				if arg == "--output" && i+1 < len(args) {
					outputPath = args[i+1]
					break
				}
			}

			if outputPath != "" {
				g.Expect(output).To(ContainSubstring("decrypted data written to"))

				// Verify the output file exists and contains the original data
				decryptedData, err := os.ReadFile(outputPath)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(string(decryptedData)).To(Equal("secret test data"))
			}
		})
	}
}

func TestDistroDecryptTokenCmdWithEnvVar(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	privateKeySetPath := filepath.Join(tempDir, "private.jwks")
	err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Read private key set content
	keySetData, err := os.ReadFile(privateKeySetPath)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable using t.Setenv for automatic cleanup
	t.Setenv(distroEncPrivateKeySetEnvVar, string(keySetData))

	// Create test data and encrypt it
	originalData := "secret data from env test"
	jweToken, err := lkm.EncryptTokenWithKeySet([]byte(originalData), publicKeySet, "")
	g.Expect(err).ToNot(HaveOccurred())

	inputPath := filepath.Join(tempDir, "encrypted.jwe")
	err = os.WriteFile(inputPath, []byte(jweToken), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	outputPath := filepath.Join(tempDir, "decrypted.txt")

	// Execute command without --key-set flag (should use env var)
	args := []string{"distro", "decrypt", "token", "--input", inputPath, "--output", outputPath}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("decrypted data written to"))

	// Verify the output file exists and contains the original data
	decryptedData, err := os.ReadFile(outputPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(decryptedData)).To(Equal(originalData))
}

func TestDistroEncryptDecryptRoundTrip(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	publicKeySetPath := filepath.Join(tempDir, "public.jwks")
	err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	privateKeySetPath := filepath.Join(tempDir, "private.jwks")
	err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Create test data
	originalData := "This is a comprehensive round-trip test with special chars: !@#$%^&*()_+-={}[]|\\:;\"'<>,.?/"
	inputPath := filepath.Join(tempDir, "original.txt")
	err = os.WriteFile(inputPath, []byte(originalData), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	encryptedPath := filepath.Join(tempDir, "encrypted.jwe")
	decryptedPath := filepath.Join(tempDir, "decrypted.txt")

	// Step 1: Encrypt the data
	encryptArgs := []string{"distro", "encrypt", "token", "--key-set", publicKeySetPath, "--input", inputPath, "--output", encryptedPath}
	_, err = executeCommand(encryptArgs)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify encrypted file exists and looks like JWE
	encryptedData, err := os.ReadFile(encryptedPath)
	g.Expect(err).ToNot(HaveOccurred())
	encryptedString := string(encryptedData)
	parts := strings.Split(encryptedString, ".")
	g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")

	// Step 2: Decrypt the data
	decryptArgs := []string{"distro", "decrypt", "token", "--key-set", privateKeySetPath, "--input", encryptedPath, "--output", decryptedPath}
	_, err = executeCommand(decryptArgs)
	g.Expect(err).ToNot(HaveOccurred())

	// Step 3: Verify the decrypted data matches the original
	decryptedData, err := os.ReadFile(decryptedPath)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(decryptedData)).To(Equal(originalData))
}
