// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

func TestDistroRevokeLicenseKeyCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "successfully revokes license key",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				// Create a license key
				licenseFile := filepath.Join(tempDir, "license.jwt")
				_, err = executeCommand([]string{
					"distro", "sign", "license-key",
					"--customer", "Test Company",
					"--duration", "30",
					"--key-set", privateKeyFile,
					"--output", licenseFile,
				})
				if err != nil {
					return nil, err
				}

				// Revoke the license key
				revocationFile := filepath.Join(tempDir, "keys.rks")
				return []string{
					"distro", "revoke", "license-key", licenseFile,
					"--output", revocationFile,
				}, nil
			},
			expectError: false,
		},
		{
			name: "fails without output flag",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys and license
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				licenseFile := filepath.Join(tempDir, "license.jwt")
				_, err = executeCommand([]string{
					"distro", "sign", "license-key",
					"--customer", "Test Company",
					"--duration", "30",
					"--key-set", privateKeyFile,
					"--output", licenseFile,
				})
				if err != nil {
					return nil, err
				}

				return []string{
					"distro", "revoke", "license-key", licenseFile,
				}, nil
			},
			expectError:  true,
			errorMessage: "--output flag is required",
		},
		{
			name: "fails with non-existent license file",
			setupFunc: func(tempDir string) ([]string, error) {
				nonExistentFile := filepath.Join(tempDir, "non-existent.jwt")
				revocationFile := filepath.Join(tempDir, "keys.rks")
				return []string{
					"distro", "revoke", "license-key", nonExistentFile,
					"--output", revocationFile,
				}, nil
			},
			expectError:  true,
			errorMessage: "failed to read the license key",
		},
		{
			name: "fails with invalid license file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Create invalid license file
				invalidLicenseFile := filepath.Join(tempDir, "invalid.jwt")
				err := os.WriteFile(invalidLicenseFile, []byte("invalid jwt content"), 0644)
				if err != nil {
					return nil, err
				}

				revocationFile := filepath.Join(tempDir, "keys.rks")
				return []string{
					"distro", "revoke", "license-key", invalidLicenseFile,
					"--output", revocationFile,
				}, nil
			},
			expectError:  true,
			errorMessage: "failed to extract license ID from token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
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
			g.Expect(output).To(ContainSubstring("license key"))
			g.Expect(output).To(ContainSubstring("revoked and saved to:"))

			// Find the revocation file from args
			revocationFile := ""
			for i, arg := range args {
				if arg == "--output" && i+1 < len(args) {
					revocationFile = args[i+1]
					break
				}
			}
			g.Expect(revocationFile).ToNot(BeEmpty())

			// Verify revocation file was created
			g.Expect(revocationFile).To(BeAnExistingFile())

			// Verify revocation file contents
			data, err := os.ReadFile(revocationFile)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(data).ToNot(BeEmpty())

			var rks lkm.RevocationKeySet
			err = json.Unmarshal(data, &rks)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(rks.Issuer).To(Equal("test.issuer"))
			g.Expect(rks.Keys).To(HaveLen(1))

			// Verify the key is actually revoked (has a timestamp)
			for _, timestamp := range rks.Keys {
				g.Expect(timestamp).To(BeNumerically(">", 0))
			}
		})
	}
}

func TestDistroRevokeLicenseKeyCmdMerging(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	// Generate keys
	_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	privateKeyFile, _, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())

	// Create first license key
	licenseFile1 := filepath.Join(tempDir, "license1.jwt")
	_, err = executeCommand([]string{
		"distro", "sign", "license-key",
		"--customer", "Test Company 1",
		"--duration", "30",
		"--key-set", privateKeyFile,
		"--output", licenseFile1,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Create second license key
	licenseFile2 := filepath.Join(tempDir, "license2.jwt")
	_, err = executeCommand([]string{
		"distro", "sign", "license-key",
		"--customer", "Test Company 2",
		"--duration", "30",
		"--key-set", privateKeyFile,
		"--output", licenseFile2,
	})
	g.Expect(err).ToNot(HaveOccurred())

	revocationFile := filepath.Join(tempDir, "keys.rks")

	// Revoke first license key
	_, err = executeCommand([]string{
		"distro", "revoke", "license-key", licenseFile1,
		"--output", revocationFile,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify first revocation
	data, err := os.ReadFile(revocationFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rks lkm.RevocationKeySet
	err = json.Unmarshal(data, &rks)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rks.Keys).To(HaveLen(1))

	// Revoke second license key (should merge)
	_, err = executeCommand([]string{
		"distro", "revoke", "license-key", licenseFile2,
		"--output", revocationFile,
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify both keys are revoked
	data, err = os.ReadFile(revocationFile)
	g.Expect(err).ToNot(HaveOccurred())

	err = json.Unmarshal(data, &rks)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rks.Issuer).To(Equal("test.issuer"))
	g.Expect(rks.Keys).To(HaveLen(2))

	// Verify both keys have timestamps
	for _, timestamp := range rks.Keys {
		g.Expect(timestamp).To(BeNumerically(">", 0))
	}
}
