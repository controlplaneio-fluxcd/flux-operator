// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/gomega"
)

func TestDistroSignManifestsCmd(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "valid manifest signing",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				// Create some manifest files to sign
				manifestContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  key: value`
				err = os.WriteFile(filepath.Join(tempDir, "manifest.yaml"), []byte(manifestContent), 0644)
				if err != nil {
					return nil, err
				}

				// Find the private key file
				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				signatureFile := filepath.Join(tempDir, "signature.sig")
				return []string{"distro", "sign", "manifests", tempDir, "--key-set", privateKeyFile, "--attestation", signatureFile}, nil
			},
			expectError: false,
		},
		{
			name: "missing attestation flag",
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

				return []string{"distro", "sign", "manifests", tempDir, "--key-set", privateKeyFile}, nil
			},
			expectError:  true,
			errorMessage: "--attestation flag is required",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				signatureFile := filepath.Join(tempDir, "signature.sig")
				return []string{"distro", "sign", "manifests", tempDir, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "JWKS must be specified",
		},
		{
			name: "invalid directory",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate keys first
				_, err := executeCommand([]string{"distro", "keygen", "sig", "test.issuer", "--output-dir", tempDir})
				if err != nil {
					return nil, err
				}

				privateKeyFile, _, err := findKeyFiles(tempDir)
				if err != nil {
					return nil, err
				}

				signatureFile := filepath.Join(tempDir, "signature.sig")
				nonExistentDir := filepath.Join(tempDir, "nonexistent")
				return []string{"distro", "sign", "manifests", nonExistentDir, "--key-set", privateKeyFile, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "does not exist",
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

				signatureFile := filepath.Join(tempDir, "signature.sig")
				return []string{"distro", "sign", "manifests", tempDir, "--key-set", invalidKeyFile, "--attestation", signatureFile}, nil
			},
			expectError:  true,
			errorMessage: "failed to unmarshal",
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
			g.Expect(output).To(ContainSubstring("checksum:"))
			g.Expect(output).To(ContainSubstring("attestation written to:"))

			// Verify signature file was created
			signatureFile := ""
			for _, arg := range args {
				if strings.HasSuffix(arg, ".sig") {
					signatureFile = arg
					break
				}
			}
			g.Expect(signatureFile).ToNot(BeEmpty())

			// Check signature file exists and has content
			sigData, err := os.ReadFile(signatureFile)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(sigData).ToNot(BeEmpty())

			// Verify it's a JWT format and parse it with go-jose
			sigString := string(sigData)

			// Parse the JWT using go-jose
			jws, err := jose.ParseSigned(sigString, []jose.SignatureAlgorithm{jose.EdDSA})
			g.Expect(err).ToNot(HaveOccurred(), "signature should be a valid JWT")
			g.Expect(jws.Signatures).To(HaveLen(1), "should have exactly one signature")
			g.Expect(jws.Signatures[0].Header.Algorithm).To(Equal("EdDSA"), "signature should use EdDSA algorithm")
			g.Expect(jws.Signatures[0].Header.KeyID).ToNot(BeEmpty(), "signature should have a key ID")

			// Verify payload contains expected claims structure
			var claims map[string]any
			err = json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &claims)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(claims).To(HaveKey("iss"), "should have issuer claim")
			g.Expect(claims).To(HaveKey("sub"), "should have subject claim (checksum)")
			g.Expect(claims).To(HaveKey("aud"), "should have audience claim")
			g.Expect(claims).To(HaveKey("iat"), "should have issued at claim")
			g.Expect(claims["aud"]).To(Equal([]any{"flux-operator"}), "audience should be flux-operator")
		})
	}
}

func TestDistroSignManifestsWithEnvVar(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate keys first
	_, err := executeCommand([]string{"distro", "keygen", "sig", "env.test.issuer", "--output-dir", tempDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find and read the private key file
	privateKeyFile, _, err := findKeyFiles(tempDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(privateKeyFile).ToNot(BeEmpty())

	// Read the private key content
	privateKeyData, err := os.ReadFile(privateKeyFile)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable
	t.Setenv(distroSigPrivateKeySetEnvVar, string(privateKeyData))

	// Create manifest file
	manifestContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1`
	err = os.WriteFile(filepath.Join(tempDir, "deployment.yaml"), []byte(manifestContent), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	signatureFile := filepath.Join(tempDir, "env-signature.sig")
	args := []string{"distro", "sign", "manifests", tempDir, "--attestation", signatureFile}

	// Execute command (should use env var for key)
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("processing files:"))
	g.Expect(output).To(ContainSubstring("deployment.yaml"))
	g.Expect(output).To(ContainSubstring("attestation written to:"))

	// Verify signature file was created
	sigData, err := os.ReadFile(signatureFile)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(sigData).ToNot(BeEmpty())

	// Verify JWT format using go-jose
	sigString := string(sigData)
	jws, err := jose.ParseSigned(sigString, []jose.SignatureAlgorithm{jose.EdDSA})
	g.Expect(err).ToNot(HaveOccurred(), "signature should be a valid JWT")
	g.Expect(jws.Signatures).To(HaveLen(1), "should have exactly one signature")
	g.Expect(jws.Signatures[0].Header.Algorithm).To(Equal("EdDSA"), "signature should use EdDSA algorithm")
}

func TestDistroSignManifestsSignatureExclusion(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()
	keyDir := t.TempDir()

	// Generate keys in separate directory
	_, err := executeCommand([]string{"distro", "keygen", "sig", "exclusion.test.issuer", "--output-dir", keyDir})
	g.Expect(err).ToNot(HaveOccurred())

	// Find the private key file
	privateKeyFile, _, err := findKeyFiles(keyDir)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(privateKeyFile).ToNot(BeEmpty())

	// Create manifest files in the main temp directory
	manifest1 := `apiVersion: v1
kind: Service
metadata:
  name: test-service`
	err = os.WriteFile(filepath.Join(tempDir, "service.yaml"), []byte(manifest1), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Create the signature file in the same directory as manifests
	signatureFile := filepath.Join(tempDir, "signature.sig")

	// First signing
	args1 := []string{"distro", "sign", "manifests", tempDir, "--key-set", privateKeyFile, "--attestation", signatureFile}
	_, err = executeCommand(args1)
	g.Expect(err).ToNot(HaveOccurred())

	// Extract checksum from first signature JWT
	sigData1, err := os.ReadFile(signatureFile)
	g.Expect(err).ToNot(HaveOccurred())
	jws1, err := jose.ParseSigned(string(sigData1), []jose.SignatureAlgorithm{jose.EdDSA})
	g.Expect(err).ToNot(HaveOccurred())
	var claims1 map[string]any
	err = json.Unmarshal(jws1.UnsafePayloadWithoutVerification(), &claims1)
	g.Expect(err).ToNot(HaveOccurred())
	checksum1, ok := claims1["sub"].(string)
	g.Expect(ok).To(BeTrue(), "subject claim should be a string")
	g.Expect(checksum1).ToNot(BeEmpty())

	// Second signing (signature file already exists, should be excluded from hash)
	output2, err := executeCommand(args1)
	g.Expect(err).ToNot(HaveOccurred())

	// Extract checksum from second signature JWT
	sigData2, err := os.ReadFile(signatureFile)
	g.Expect(err).ToNot(HaveOccurred())
	jws2, err := jose.ParseSigned(string(sigData2), []jose.SignatureAlgorithm{jose.EdDSA})
	g.Expect(err).ToNot(HaveOccurred())
	var claims2 map[string]any
	err = json.Unmarshal(jws2.UnsafePayloadWithoutVerification(), &claims2)
	g.Expect(err).ToNot(HaveOccurred())
	checksum2, ok := claims2["sub"].(string)
	g.Expect(ok).To(BeTrue(), "subject claim should be a string")
	g.Expect(checksum2).ToNot(BeEmpty())

	// Checksums should be the same because signature file is excluded
	g.Expect(checksum1).To(Equal(checksum2), "checksums should be identical when signature file is excluded")

	// Verify that service.yaml is processed but signature.sig is not in the processing files list
	g.Expect(output2).To(ContainSubstring("service.yaml"), "service.yaml should be processed")

	// Extract the processing files section and verify signature.sig is not listed there
	processingSection := strings.Split(output2, "processing files:")[1]
	processingSection = strings.Split(processingSection, "checksum:")[0]
	g.Expect(processingSection).ToNot(ContainSubstring("signature.sig"), "signature file should be excluded from processing files list")
}
