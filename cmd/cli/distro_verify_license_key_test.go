// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDistroVerifyLicenseKeyValidLicense(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	// Generate keys first
	_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find the key files
	privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())

	licenseFile := filepath.Join(tempDir, "license.jwt")

	// Sign a license key with 30 days duration
	_, err = executeCommand([]string{"distro", "sign", "license-key", "--customer", "Test Company LLC", "--duration", "30", "--key-set", privateKeyFile, "--output", licenseFile})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify the license key
	output, err := executeCommand([]string{"distro", "verify", "license-key", licenseFile, "--key-set", publicKeyFile})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("✔ license key is issued by test.issuer"))
	g.Expect(output).To(ContainSubstring("✔ license key is valid until"))
}

func TestDistroVerifyLicenseKeyExpiredLicense(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	// Generate keys first
	_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find the key files
	privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())

	licenseFile := filepath.Join(tempDir, "expired-license.jwt")

	// Sign a license key with negative duration (expired)
	output, err := executeCommand([]string{"distro", "sign", "lk", "-c", "Expired Company", "-d", "-1", "-k", privateKeyFile, "-o", licenseFile})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("warning: negative duration will result in an expired license key"))

	// Try to verify the expired license key
	_, err = executeCommand([]string{"distro", "verify", "lk", licenseFile, "-k", publicKeyFile})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("license key has expired"))
}

func TestDistroVerifyLicenseKeyWithEnvironmentVariable(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	// Generate keys first
	_, err := executeCommand([]string{"distro", "keygen", "env.test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find the key files
	privateKeyFile, publicKeyFile, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())

	licenseFile := filepath.Join(tempDir, "license.jwt")

	// Sign a license key
	_, err = executeCommand([]string{"distro", "sign", "license-key", "--customer", "Env Test Company", "--duration", "7", "--key-set", privateKeyFile, "--output", licenseFile})
	g.Expect(err).ToNot(HaveOccurred())

	// Read public key set
	publicKeyData, err := os.ReadFile(publicKeyFile)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable
	t.Setenv(distroPublicKeySetEnvVar, string(publicKeyData))

	// Verify the license key using environment variable
	output, err := executeCommand([]string{"distro", "verify", "license-key", licenseFile})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("✔ license key is issued by env.test.issuer"))
	g.Expect(output).To(ContainSubstring("✔ license key is valid until"))
}

func TestDistroVerifyLicenseKeyErrorCases(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "missing license file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the public key file
				_, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}
				return []string{"distro", "verify", "license-key", "nonexistent.jwt", "--key-set", publicKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to read license file",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the key files
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				licenseFile := filepath.Join(tempDir, "license.jwt")

				// Create a license
				_, err = executeCommand([]string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--key-set", privateKeyFile, "--output", licenseFile})
				if err != nil {
					return nil, err
				}

				return []string{"distro", "verify", "license-key", licenseFile}, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "invalid license file content",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the public key file
				_, publicKeyFile, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				licenseFile := filepath.Join(tempDir, "invalid.jwt")

				// Create invalid license file
				err = os.WriteFile(licenseFile, []byte("invalid-jwt-content"), 0644)
				if err != nil {
					return nil, err
				}

				return []string{"distro", "verify", "license-key", licenseFile, "--key-set", publicKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse signed license key",
		},
		{
			name: "wrong key set",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate first key set
				_, err := executeCommand([]string{"distro", "keygen", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the first key files
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Generate second key set in a different directory
				wrongKeyDir := filepath.Join(tempDir, "wrong")
				err = os.MkdirAll(wrongKeyDir, 0755)
				if err != nil {
					return nil, err
				}

				_, err = executeCommand([]string{"distro", "keygen", "wrong.issuer", "--output-dir", wrongKeyDir})
				if err != nil {
					return nil, err
				}

				// Find the wrong public key file
				_, wrongPublicKeyFile, err := findKeyFiles(wrongKeyDir)
				if err != nil {
					return nil, err
				}

				licenseFile := filepath.Join(tempDir, "license.jwt")

				// Create license with first key set
				_, err = executeCommand([]string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--key-set", privateKeyFile, "--output", licenseFile})
				if err != nil {
					return nil, err
				}

				// Try to verify with wrong key set
				return []string{"distro", "verify", "license-key", licenseFile, "--key-set", wrongPublicKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "key with ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tempDir := t.TempDir()

			args, err := tt.setupFunc(tempDir)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = executeCommand(args)
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errorMessage != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errorMessage))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}
