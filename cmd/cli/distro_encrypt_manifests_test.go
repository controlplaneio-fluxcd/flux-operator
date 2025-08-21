// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

func TestDistroEncryptManifestsCmd(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "encrypt manifests with default ignore patterns",
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

				// Create test manifests
				manifestsDir := filepath.Join(tempDir, "manifests")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				// Create some test files
				testFiles := map[string]string{
					"deployment.yaml": "apiVersion: apps/v1\nkind: Deployment",
					"service.yaml":    "apiVersion: v1\nkind: Service",
					"config.jws":      "should.be.ignored.by.default",
				}

				for filename, content := range testFiles {
					err = os.WriteFile(filepath.Join(manifestsDir, filename), []byte(content), 0644)
					if err != nil {
						return nil, err
					}
				}

				outputPath := filepath.Join(tempDir, "manifests.zip.jwe")

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", keySetPath, "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "encrypt manifests with custom ignore patterns",
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

				// Create test manifests with subdirectory
				manifestsDir := filepath.Join(tempDir, "manifests")
				subDir := filepath.Join(manifestsDir, "subdir")
				err = os.MkdirAll(subDir, 0755)
				if err != nil {
					return nil, err
				}

				// Create test files
				testFiles := map[string]string{
					"deployment.yaml":    "apiVersion: apps/v1\nkind: Deployment",
					"service.yaml":       "apiVersion: v1\nkind: Service",
					"secret.yaml":        "apiVersion: v1\nkind: Secret",
					"subdir/config.yaml": "config: value",
					"subdir/backup.yaml": "backup: data",
					"temp.log":           "log data",
					"debug.log":          "debug info",
				}

				for filename, content := range testFiles {
					fullPath := filepath.Join(manifestsDir, filename)
					err = os.MkdirAll(filepath.Dir(fullPath), 0755)
					if err != nil {
						return nil, err
					}
					err = os.WriteFile(fullPath, []byte(content), 0644)
					if err != nil {
						return nil, err
					}
				}

				outputPath := filepath.Join(tempDir, "manifests.zip.jwe")

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", keySetPath, "--ignore", "*.log,*secret*", "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "encrypt manifests with hidden files and directories",
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

				// Create test manifests with hidden files
				manifestsDir := filepath.Join(tempDir, "manifests")
				hiddenDir := filepath.Join(manifestsDir, ".git")
				err = os.MkdirAll(hiddenDir, 0755)
				if err != nil {
					return nil, err
				}

				// Create test files including hidden ones
				testFiles := map[string]string{
					"deployment.yaml": "apiVersion: apps/v1\nkind: Deployment",
					".gitignore":      "*.log\n*.tmp",
					".git/config":     "git config data",
					".env":            "SECRET=value",
				}

				for filename, content := range testFiles {
					fullPath := filepath.Join(manifestsDir, filename)
					err = os.MkdirAll(filepath.Dir(fullPath), 0755)
					if err != nil {
						return nil, err
					}
					err = os.WriteFile(fullPath, []byte(content), 0644)
					if err != nil {
						return nil, err
					}
				}

				outputPath := filepath.Join(tempDir, "manifests.zip.jwe")

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", keySetPath, "--ignore", ".*", "--output", outputPath}, nil
			},
			expectError: false,
		},
		{
			name: "missing output flag",
			setupFunc: func(tempDir string) ([]string, error) {
				manifestsDir := filepath.Join(tempDir, "manifests")
				err := os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", "dummy"}, nil
			},
			expectError:  true,
			errorMessage: "--output flag is required",
		},
		{
			name: "nonexistent directory",
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

				nonexistentDir := filepath.Join(tempDir, "nonexistent")
				outputPath := filepath.Join(tempDir, "output.zip.jwe")

				return []string{"distro", "encrypt", "manifests", nonexistentDir, "--key-set", keySetPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "does not exist",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				manifestsDir := filepath.Join(tempDir, "manifests")
				err := os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.zip.jwe")

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--output", outputPath}, nil
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

				manifestsDir := filepath.Join(tempDir, "manifests")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				outputPath := filepath.Join(tempDir, "output.zip.jwe")

				return []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", keySetPath, "--output", outputPath}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse public key set",
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
				g.Expect(output).To(ContainSubstring("encrypted"))

				// Verify the output file exists and contains a JWE token
				jweData, err := os.ReadFile(outputPath)
				g.Expect(err).ToNot(HaveOccurred())

				// JWE tokens have 5 parts separated by dots
				jweString := string(jweData)
				parts := strings.Split(jweString, ".")
				g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")
				g.Expect(jweString).ToNot(BeEmpty())

				// Verify that archiving output is present
				g.Expect(output).To(ContainSubstring("archiving files:"))
			}
		})
	}
}

func TestDistroEncryptManifestsCmdWithEnvVar(t *testing.T) {
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

	// Create test manifests
	manifestsDir := filepath.Join(tempDir, "manifests")
	err = os.MkdirAll(manifestsDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(manifestsDir, "test.yaml"), []byte("apiVersion: v1\nkind: Service"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	outputPath := filepath.Join(tempDir, "manifests.zip.jwe")

	// Execute command without --key-set flag (should use env var)
	args := []string{"distro", "encrypt", "manifests", manifestsDir, "--output", outputPath}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("encrypted"))

	// Verify the output file exists and contains a JWE token
	jweData, err := os.ReadFile(outputPath)
	g.Expect(err).ToNot(HaveOccurred())

	// JWE tokens have 5 parts separated by dots
	jweString := string(jweData)
	parts := strings.Split(jweString, ".")
	g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")
	g.Expect(jweString).ToNot(BeEmpty())
}

func TestDistroEncryptManifestsCmdZipContent(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	keySetPath := filepath.Join(tempDir, "public.jwks")
	err = lkm.WriteEncryptionKeySet(keySetPath, publicKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Create test manifests with subdirectories
	manifestsDir := filepath.Join(tempDir, "manifests")
	subDir := filepath.Join(manifestsDir, "apps")
	err = os.MkdirAll(subDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	testFiles := map[string]string{
		"deployment.yaml":     "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test",
		"service.yaml":        "apiVersion: v1\nkind: Service\nmetadata:\n  name: test-svc",
		"apps/configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-config",
		"ignore.jws":          "this should be ignored",
	}

	for filename, content := range testFiles {
		fullPath := filepath.Join(manifestsDir, filename)
		err = os.MkdirAll(filepath.Dir(fullPath), 0755)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(fullPath, []byte(content), 0644)
		g.Expect(err).ToNot(HaveOccurred())
	}

	outputPath := filepath.Join(tempDir, "manifests.zip.jwe")

	// Encrypt the manifests (default ignore patterns should exclude .jws files)
	args := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", keySetPath, "--output", outputPath}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())

	// Decrypt and verify the zip content
	jweData, err := os.ReadFile(outputPath)
	g.Expect(err).ToNot(HaveOccurred())

	decryptedData, err := lkm.DecryptTokenWithKeySet(jweData, privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	// Parse the zip content
	reader := bytes.NewReader(decryptedData)
	zipReader, err := zip.NewReader(reader, int64(len(decryptedData)))
	g.Expect(err).ToNot(HaveOccurred())

	// Check that expected files are present and ignored files are absent
	foundFiles := make(map[string]bool)
	for _, file := range zipReader.File {
		foundFiles[file.Name] = true
	}

	// Should contain these files
	expectedFiles := []string{"deployment.yaml", "service.yaml", "apps/", "apps/configmap.yaml"}
	for _, expectedFile := range expectedFiles {
		g.Expect(foundFiles[expectedFile]).To(BeTrue(), "Expected file %s to be in zip", expectedFile)
	}

	// Should NOT contain ignored .jws file
	g.Expect(foundFiles).ToNot(HaveKey("ignore.jws"), "JWS file should be ignored by default")

	// Verify output shows processed files
	g.Expect(output).To(ContainSubstring("archiving files:"))
	g.Expect(output).To(ContainSubstring("deployment.yaml"))
	g.Expect(output).To(ContainSubstring("service.yaml"))
	g.Expect(output).To(ContainSubstring("apps/configmap.yaml"))
}
