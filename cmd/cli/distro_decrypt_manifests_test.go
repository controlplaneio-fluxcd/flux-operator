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

func TestDistroDecryptManifestsCmd(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		name         string
		setupFunc    func(string) ([]string, error)
		expectError  bool
		errorMessage string
	}{
		{
			name: "decrypt manifests to current directory",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create test manifests and encrypt them
				manifestsDir := filepath.Join(tempDir, "source")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				testFiles := map[string]string{
					"deployment.yaml": "apiVersion: apps/v1\nkind: Deployment",
					"service.yaml":    "apiVersion: v1\nkind: Service",
				}

				for filename, content := range testFiles {
					err = os.WriteFile(filepath.Join(manifestsDir, filename), []byte(content), 0644)
					if err != nil {
						return nil, err
					}
				}

				// Encrypt the manifests using the encrypt command
				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
				encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
				_, err = executeCommand(encryptArgs)
				if err != nil {
					return nil, err
				}

				// Set up output directory for decryption
				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError: false,
		},
		{
			name: "decrypt manifests with subdirectories",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create test manifests with subdirectories
				manifestsDir := filepath.Join(tempDir, "source")
				appsDir := filepath.Join(manifestsDir, "apps")
				err = os.MkdirAll(appsDir, 0755)
				if err != nil {
					return nil, err
				}

				testFiles := map[string]string{
					"deployment.yaml":     "apiVersion: apps/v1\nkind: Deployment",
					"service.yaml":        "apiVersion: v1\nkind: Service",
					"apps/configmap.yaml": "apiVersion: v1\nkind: ConfigMap",
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

				// Encrypt the manifests
				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
				encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
				_, err = executeCommand(encryptArgs)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError: false,
		},
		{
			name: "decrypt with overwrite protection",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create and encrypt test manifests
				manifestsDir := filepath.Join(tempDir, "source")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				err = os.WriteFile(filepath.Join(manifestsDir, "test.yaml"), []byte("content"), 0644)
				if err != nil {
					return nil, err
				}

				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
				encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
				_, err = executeCommand(encryptArgs)
				if err != nil {
					return nil, err
				}

				// Create output directory with existing file
				outputDir := filepath.Join(tempDir, "output")
				err = os.MkdirAll(outputDir, 0755)
				if err != nil {
					return nil, err
				}

				err = os.WriteFile(filepath.Join(outputDir, "test.yaml"), []byte("existing content"), 0644)
				if err != nil {
					return nil, err
				}

				return []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "already exists",
		},
		{
			name: "decrypt with overwrite flag",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create and encrypt test manifests
				manifestsDir := filepath.Join(tempDir, "source")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				err = os.WriteFile(filepath.Join(manifestsDir, "test.yaml"), []byte("new content"), 0644)
				if err != nil {
					return nil, err
				}

				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
				if err != nil {
					return nil, err
				}

				encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
				encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
				_, err = executeCommand(encryptArgs)
				if err != nil {
					return nil, err
				}

				// Create output directory with existing file
				outputDir := filepath.Join(tempDir, "output")
				err = os.MkdirAll(outputDir, 0755)
				if err != nil {
					return nil, err
				}

				err = os.WriteFile(filepath.Join(outputDir, "test.yaml"), []byte("old content"), 0644)
				if err != nil {
					return nil, err
				}

				return []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir, "--overwrite"}, nil
			},
			expectError: false,
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

				nonexistentFile := filepath.Join(tempDir, "nonexistent.zip.jwe")
				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", nonexistentFile, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "does not exist",
		},
		{
			name: "missing key set",
			setupFunc: func(tempDir string) ([]string, error) {
				// Create fake encrypted file
				fakeEncryptedPath := filepath.Join(tempDir, "fake.zip.jwe")
				err := os.WriteFile(fakeEncryptedPath, []byte("fake.jwe.data"), 0644)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", fakeEncryptedPath, "--output-dir", outputDir}, nil
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

				fakeEncryptedPath := filepath.Join(tempDir, "fake.zip.jwe")
				err = os.WriteFile(fakeEncryptedPath, []byte("fake.jwe.data"), 0644)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", fakeEncryptedPath, "--key-set", keySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "failed to parse private key set",
		},
		{
			name: "invalid JWE file",
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

				invalidJWEPath := filepath.Join(tempDir, "invalid.zip.jwe")
				err = os.WriteFile(invalidJWEPath, []byte("invalid.jwe.token"), 0644)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", invalidJWEPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "failed to decrypt archive",
		},
		{
			name: "wrong private key",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate two different key pairs
				publicKeySet1, _, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				_, privateKeySet2, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				// Create and encrypt with first key pair
				manifestsDir := filepath.Join(tempDir, "source")
				err = os.MkdirAll(manifestsDir, 0755)
				if err != nil {
					return nil, err
				}

				err = os.WriteFile(filepath.Join(manifestsDir, "test.yaml"), []byte("content"), 0644)
				if err != nil {
					return nil, err
				}

				publicKeySetPath := filepath.Join(tempDir, "public.jwks")
				err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet1)
				if err != nil {
					return nil, err
				}

				encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
				encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
				_, err = executeCommand(encryptArgs)
				if err != nil {
					return nil, err
				}

				// Try to decrypt with second (wrong) key pair
				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet2)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "failed to decrypt archive",
		},
		{
			name: "zip slip vulnerability protection",
			setupFunc: func(tempDir string) ([]string, error) {
				// Generate key pair
				publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
				if err != nil {
					return nil, err
				}

				privateKeySetPath := filepath.Join(tempDir, "private.jwks")
				err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
				if err != nil {
					return nil, err
				}

				// Create a malicious zip archive with path traversal
				var buf bytes.Buffer
				zipWriter := zip.NewWriter(&buf)

				// Add a malicious file with path traversal
				maliciousFile, err := zipWriter.Create("../../../etc/passwd")
				if err != nil {
					return nil, err
				}
				_, err = maliciousFile.Write([]byte("malicious content"))
				if err != nil {
					return nil, err
				}

				err = zipWriter.Close()
				if err != nil {
					return nil, err
				}

				// Encrypt the malicious zip
				jweToken, err := lkm.EncryptTokenWithKeySet(buf.Bytes(), publicKeySet, "")
				if err != nil {
					return nil, err
				}

				maliciousJWEPath := filepath.Join(tempDir, "malicious.zip.jwe")
				err = os.WriteFile(maliciousJWEPath, []byte(jweToken), 0644)
				if err != nil {
					return nil, err
				}

				outputDir := filepath.Join(tempDir, "output")

				return []string{"distro", "decrypt", "manifests", maliciousJWEPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}, nil
			},
			expectError:  true,
			errorMessage: "failed to extract file",
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

			// Check if output contains expected messages
			g.Expect(output).To(ContainSubstring("extracted files:"))
			g.Expect(output).To(ContainSubstring("extracted"))
			g.Expect(output).To(ContainSubstring("files to:"))

			// Verify files were actually extracted
			var outputDir string
			for i, arg := range args {
				if arg == "--output-dir" && i+1 < len(args) {
					outputDir = args[i+1]
					break
				}
			}

			if outputDir != "" {
				// Check that output directory exists
				_, err := os.Stat(outputDir)
				g.Expect(err).ToNot(HaveOccurred())

				// Check for expected files (at least some should exist)
				entries, err := os.ReadDir(outputDir)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(entries).ToNot(BeEmpty(), "Output directory should contain extracted files")
			}
		})
	}
}

func TestDistroDecryptManifestsCmdWithEnvVar(t *testing.T) {
	g := NewWithT(t)

	tempDir := t.TempDir()

	// Generate key pair
	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	// Create and encrypt test manifests
	manifestsDir := filepath.Join(tempDir, "source")
	err = os.MkdirAll(manifestsDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(manifestsDir, "test.yaml"), []byte("test: value"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	publicKeySetPath := filepath.Join(tempDir, "public.jwks")
	err = lkm.WriteEncryptionKeySet(publicKeySetPath, publicKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
	encryptArgs := []string{"distro", "encrypt", "manifests", manifestsDir, "--key-set", publicKeySetPath, "--ignore", "", "--output", encryptedPath}
	_, err = executeCommand(encryptArgs)
	g.Expect(err).ToNot(HaveOccurred())

	// Set up private key set environment variable
	privateKeySetPath := filepath.Join(tempDir, "private.jwks")
	err = lkm.WriteEncryptionKeySet(privateKeySetPath, privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())

	keySetData, err := os.ReadFile(privateKeySetPath)
	g.Expect(err).ToNot(HaveOccurred())

	// Set environment variable using t.Setenv for automatic cleanup
	t.Setenv(distroEncPrivateKeySetEnvVar, string(keySetData))

	outputDir := filepath.Join(tempDir, "output")

	// Execute command without --key-set flag (should use env var)
	args := []string{"distro", "decrypt", "manifests", encryptedPath, "--output-dir", outputDir}
	output, err := executeCommand(args)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(output).To(ContainSubstring("extracted files:"))

	// Verify extraction worked
	extractedFile := filepath.Join(outputDir, "test.yaml")
	content, err := os.ReadFile(extractedFile)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(string(content)).To(Equal("test: value"))
}

func TestDistroEncryptDecryptManifestsRoundTrip(t *testing.T) {
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

	// Create test manifests with complex structure
	sourceDir := filepath.Join(tempDir, "source")
	subDirs := []string{
		filepath.Join(sourceDir, "apps"),
		filepath.Join(sourceDir, "config"),
		filepath.Join(sourceDir, "config/secret"),
	}

	for _, dir := range subDirs {
		err = os.MkdirAll(dir, 0755)
		g.Expect(err).ToNot(HaveOccurred())
	}

	testFiles := map[string]string{
		"deployment.yaml":           "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-app\n  namespace: default",
		"service.yaml":              "apiVersion: v1\nkind: Service\nmetadata:\n  name: test-svc\n  namespace: default",
		"apps/configmap.yaml":       "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\ndata:\n  key: value",
		"config/secret/secret.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: app-secret\nstringData:\n  password: secret123",
		"README.md":                 "# Test Application\n\nThis is a test application with Kubernetes manifests.",
		"ignore-me.jws":             "this.should.be.ignored.by.default",
	}

	for filename, content := range testFiles {
		fullPath := filepath.Join(sourceDir, filename)
		err = os.MkdirAll(filepath.Dir(fullPath), 0755)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(fullPath, []byte(content), 0644)
		g.Expect(err).ToNot(HaveOccurred())
	}

	encryptedPath := filepath.Join(tempDir, "manifests.zip.jwe")
	outputDir := filepath.Join(tempDir, "output")

	// Step 1: Encrypt the manifests (using default ignore patterns)
	encryptArgs := []string{"distro", "encrypt", "manifests", sourceDir, "--key-set", publicKeySetPath, "--output", encryptedPath}
	encryptOutput, err := executeCommand(encryptArgs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(encryptOutput).To(ContainSubstring("encrypted"))

	// Verify encrypted file exists and is JWE
	jweData, err := os.ReadFile(encryptedPath)
	g.Expect(err).ToNot(HaveOccurred())
	jweString := string(jweData)
	parts := strings.Split(jweString, ".")
	g.Expect(parts).To(HaveLen(5), "JWE token should have 5 parts")

	// Step 2: Decrypt the manifests
	decryptArgs := []string{"distro", "decrypt", "manifests", encryptedPath, "--key-set", privateKeySetPath, "--output-dir", outputDir}
	decryptOutput, err := executeCommand(decryptArgs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decryptOutput).To(ContainSubstring("extracted files:"))

	// Step 3: Verify all files were extracted correctly (except ignored ones)
	expectedFiles := map[string]string{
		"deployment.yaml":           testFiles["deployment.yaml"],
		"service.yaml":              testFiles["service.yaml"],
		"apps/configmap.yaml":       testFiles["apps/configmap.yaml"],
		"config/secret/secret.yaml": testFiles["config/secret/secret.yaml"],
		"README.md":                 testFiles["README.md"],
	}

	for filename, expectedContent := range expectedFiles {
		extractedPath := filepath.Join(outputDir, filename)
		actualContent, err := os.ReadFile(extractedPath)
		g.Expect(err).ToNot(HaveOccurred(), "File %s should exist", filename)
		g.Expect(string(actualContent)).To(Equal(expectedContent), "Content of %s should match", filename)
	}

	// Verify that ignored .jws file was NOT extracted
	ignoredPath := filepath.Join(outputDir, "ignore-me.jws")
	_, err = os.Stat(ignoredPath)
	g.Expect(os.IsNotExist(err)).To(BeTrue(), "JWS file should not be extracted")

	// Verify directory structure is preserved
	dirs := []string{
		filepath.Join(outputDir, "apps"),
		filepath.Join(outputDir, "config"),
		filepath.Join(outputDir, "config", "secret"),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		g.Expect(err).ToNot(HaveOccurred(), "Directory %s should exist", dir)
		g.Expect(info.IsDir()).To(BeTrue(), "%s should be a directory", dir)
	}
}
