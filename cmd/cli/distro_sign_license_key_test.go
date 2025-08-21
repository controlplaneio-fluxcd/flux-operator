// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/gomega"
)

func TestDistroSignLicenseKeyCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid license key signing",
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

				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--key-set", privateKeyFile, "--output", outputFile}, nil
			},
			expectError: false,
		},
		{
			name: "missing customer flag",
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

				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--duration", "30", "--key-set", privateKeyFile, "--output", outputFile}, nil
			},
			expectError:  true,
			errorMessage: "--customer flag is required",
		},
		{
			name: "missing duration flag",
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

				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--key-set", privateKeyFile, "--output", outputFile}, nil
			},
			expectError:  true,
			errorMessage: "--duration flag is required",
		},
		{
			name: "zero duration",
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

				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "0", "--key-set", privateKeyFile, "--output", outputFile}, nil
			},
			expectError:  true,
			errorMessage: "--duration flag is required",
		},
		{
			name: "missing output flag",
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

				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--key-set", privateKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "--output flag is required",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--output", outputFile}, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "invalid key file",
			setupFunc: func(tempDir string) ([]string, error) {
				// Create invalid key file
				invalidKeyFile := filepath.Join(tempDir, "invalid.jwks")
				err := os.WriteFile(invalidKeyFile, []byte("invalid json"), 0644)
				if err != nil {
					return nil, err
				}

				outputFile := filepath.Join(tempDir, "license.jwt")
				return []string{"distro", "sign", "license-key", "--customer", "Test Company", "--duration", "30", "--key-set", invalidKeyFile, "--output", outputFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to unmarshal",
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
			g.Expect(output).To(ContainSubstring("license key written to:"))

			// Find output file from args
			outputFile := ""
			for i, arg := range args {
				if arg == "--output" && i+1 < len(args) {
					outputFile = args[i+1]
					break
				}
			}
			g.Expect(outputFile).ToNot(BeEmpty())

			// Verify license key file was created
			licenseData, err := os.ReadFile(outputFile)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(licenseData).ToNot(BeEmpty())

			// Verify it's a JWT format and parse it with go-jose
			licenseString := string(licenseData)

			// Parse the JWT using go-jose
			jws, err := jose.ParseSigned(licenseString, []jose.SignatureAlgorithm{jose.EdDSA})
			g.Expect(err).ToNot(HaveOccurred(), "license key should be a valid JWT")
			g.Expect(jws.Signatures).To(HaveLen(1), "should have exactly one signature")
			g.Expect(jws.Signatures[0].Header.Algorithm).To(Equal("EdDSA"), "signature should use EdDSA algorithm")
			g.Expect(jws.Signatures[0].Header.KeyID).ToNot(BeEmpty(), "signature should have a key ID")

			// Verify payload contains expected claims structure
			var claims map[string]any
			err = json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &claims)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(claims["aud"]).To(Equal(distroDefaultAudience))
			g.Expect(claims).To(HaveKey("iss"), "should have issuer claim")
			g.Expect(claims).To(HaveKey("sub"), "should have subject claim")
			g.Expect(claims).To(HaveKey("aud"), "should have audience claim")
			g.Expect(claims).To(HaveKey("iat"), "should have issued at claim")
			g.Expect(claims).To(HaveKey("exp"), "should have expiration claim")
			g.Expect(claims).ToNot(HaveKey("caps"), "should not have capabilities claim")

			// Verify subject starts with "c-"
			subject, ok := claims["sub"].(string)
			g.Expect(ok).To(BeTrue(), "subject claim should be a string")
			g.Expect(subject).To(HavePrefix("c-"), "subject should start with 'c-'")

			// Verify expiration is in the future
			iat, ok := claims["iat"].(float64)
			g.Expect(ok).To(BeTrue(), "iat claim should be a number")
			exp, ok := claims["exp"].(float64)
			g.Expect(ok).To(BeTrue(), "exp claim should be a number")

			expectedExpiration := int64(iat) + int64(29*24*60*60) // 29 days in seconds
			g.Expect(int64(exp)).To(BeNumerically(">", expectedExpiration))
		})
	}
}

func TestDistroSignLicenseKeyCmdCapabilities(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	// Generate keys first
	_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find the private key file
	privateKeyFile, _, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())

	outputFile := filepath.Join(tempDir, "license.jwt")

	// Test with capabilities flag
	args := []string{
		"distro", "sign", "license-key",
		"--customer", "Test Company",
		"--duration", "30",
		"--capabilities", "feature1,feature2,feature3",
		"--key-set", privateKeyFile,
		"--output", outputFile,
	}

	// Execute command
	_, err = executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify license key file was created
	licenseData, err := os.ReadFile(outputFile)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(licenseData).ToNot(BeEmpty())

	// Parse the JWT using go-jose
	licenseString := string(licenseData)
	jws, err := jose.ParseSigned(licenseString, []jose.SignatureAlgorithm{jose.EdDSA})
	g.Expect(err).ToNot(HaveOccurred(), "license key should be a valid JWT")

	// Verify payload contains capabilities
	var claims map[string]any
	err = json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &claims)
	g.Expect(err).ToNot(HaveOccurred())

	// Check that capabilities claim exists and has the expected values
	g.Expect(claims).To(HaveKey("caps"), "should have capabilities claim")

	capabilities, ok := claims["caps"].([]any)
	g.Expect(ok).To(BeTrue(), "capabilities claim should be an array")
	g.Expect(capabilities).To(HaveLen(3), "should have 3 capabilities")

	capStrings := make([]string, len(capabilities))
	for i, c := range capabilities {
		capStrings[i] = c.(string)
	}
	g.Expect(capStrings).To(ContainElements("feature1", "feature2", "feature3"))
}
