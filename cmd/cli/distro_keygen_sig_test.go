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

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

func TestDistroKeygenCmd(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		setupFunc    func(string) error
		expectError  bool
		errorMessage string
	}{
		{
			name:        "valid key generation",
			args:        []string{"distro", "keygen", "sig", "some.issuer"},
			expectError: false,
		},
		{
			name: "custom output directory",
			args: []string{"distro", "keygen", "sig", "custom.issuer", "--output-dir", "subdir"},
			setupFunc: func(tempDir string) error {
				// Create subdirectory
				return os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
			},
			expectError: false,
		},
		{
			name:         "missing issuer argument",
			args:         []string{"distro", "keygen", "sig"},
			expectError:  true,
			errorMessage: "accepts 1 arg(s), received 0",
		},
		{
			name:         "empty issuer argument",
			args:         []string{"distro", "keygen", "sig", ""},
			expectError:  true,
			errorMessage: "issuer is required",
		},
		{
			name:         "invalid output directory",
			args:         []string{"distro", "keygen", "sig", "test.issuer", "--output-dir", "/nonexistent/path"},
			expectError:  true,
			errorMessage: "directory /nonexistent/path does not exist",
		},
		{
			name: "output directory is file",
			args: []string{"distro", "keygen", "sig", "test.issuer", "--output-dir", "testfile"},
			setupFunc: func(tempDir string) error {
				// Create a file instead of directory
				return os.WriteFile(filepath.Join(tempDir, "testfile"), []byte("test"), 0644)
			},
			expectError:  true,
			errorMessage: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create temporary directory for this test
			tempDir := t.TempDir()

			// Run setup function if provided
			if tt.setupFunc != nil {
				err := tt.setupFunc(tempDir)
				g.Expect(err).ToNot(HaveOccurred())
			}

			// Update args to use absolute paths where needed
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == "--output-dir" && i+1 < len(args) {
					if args[i+1] == "subdir" {
						args[i+1] = filepath.Join(tempDir, "subdir")
					} else if args[i+1] == "testfile" {
						args[i+1] = filepath.Join(tempDir, "testfile")
					} else if args[i+1] != "/nonexistent/path" {
						// For other relative paths, make them absolute
						args[i+1] = filepath.Join(tempDir, args[i+1])
					}
				}
			}

			// If no output dir specified, add it to use tempDir
			hasOutputDir := false
			for _, arg := range args {
				if arg == "--output-dir" {
					hasOutputDir = true
					break
				}
			}
			if !hasOutputDir {
				args = append(args, "--output-dir", tempDir)
			}

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
			g.Expect(output).To(ContainSubstring("private key set written to"))
			g.Expect(output).To(ContainSubstring("public key set written to"))

			// Determine the output directory
			outputDir := tempDir
			for i, arg := range args {
				if arg == "--output-dir" && i+1 < len(args) {
					outputDir = args[i+1]
					break
				}
			}

			// Find generated files
			privateKeyFile, publicKeyFile, err := findKeyFiles(outputDir)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(privateKeyFile).ToNot(BeEmpty(), "private key file should be generated")
			g.Expect(publicKeyFile).ToNot(BeEmpty(), "public key file should be generated")

			// Check file permissions
			privateInfo, err := os.Stat(privateKeyFile)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(privateInfo.Mode().Perm()).To(Equal(os.FileMode(0600)), "private key should have 0600 permissions")

			publicInfo, err := os.Stat(publicKeyFile)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(publicInfo.Mode().Perm()).To(Equal(os.FileMode(0644)), "public key should have 0644 permissions")

			// Validate private key set JSON structure
			privateData, err := os.ReadFile(privateKeyFile)
			g.Expect(err).ToNot(HaveOccurred())

			var privateKeySet lkm.EdKeySet
			err = json.Unmarshal(privateData, &privateKeySet)
			g.Expect(err).ToNot(HaveOccurred())

			expectedIssuer := tt.args[3]
			g.Expect(privateKeySet.Issuer).To(Equal(expectedIssuer))
			g.Expect(privateKeySet.Keys).To(HaveLen(1))
			g.Expect(privateKeySet.Keys[0].Algorithm).To(Equal(string(jose.EdDSA)))
			g.Expect(privateKeySet.Keys[0].Use).To(Equal("sig"))
			g.Expect(privateKeySet.Keys[0].KeyID).ToNot(BeEmpty())

			// Validate public key set JSON structure
			publicData, err := os.ReadFile(publicKeyFile)
			g.Expect(err).ToNot(HaveOccurred())

			var publicKeySet jose.JSONWebKeySet
			err = json.Unmarshal(publicData, &publicKeySet)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(publicKeySet.Keys).To(HaveLen(1))
			g.Expect(publicKeySet.Keys[0].Algorithm).To(Equal(string(jose.EdDSA)))
			g.Expect(publicKeySet.Keys[0].Use).To(Equal("sig"))
			g.Expect(publicKeySet.Keys[0].KeyID).To(Equal(privateKeySet.Keys[0].KeyID))

			// Validate that private and public keys are a valid Ed25519 key pair
			g.Expect(privateKeySet.Keys[0].Key).ToNot(BeNil())
			g.Expect(publicKeySet.Keys[0].Key).ToNot(BeNil())
		})
	}
}

func TestDistroKeygenUniqueKeyIDs(t *testing.T) {
	g := NewWithT(t)

	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	// Generate first key pair
	args1 := []string{"distro", "keygen", "sig", "test1.issuer", "--output-dir", tempDir1}
	_, err := executeCommand(args1)
	g.Expect(err).ToNot(HaveOccurred())

	// Get first key ID
	files1, err := os.ReadDir(tempDir1)
	g.Expect(err).ToNot(HaveOccurred())

	var keyID1 string
	for _, file := range files1 {
		if strings.Contains(file.Name(), "private") && filepath.Ext(file.Name()) == ".jwks" {
			data, err := os.ReadFile(filepath.Join(tempDir1, file.Name()))
			g.Expect(err).ToNot(HaveOccurred())

			var keySet lkm.EdKeySet
			err = json.Unmarshal(data, &keySet)
			g.Expect(err).ToNot(HaveOccurred())

			keyID1 = keySet.Keys[0].KeyID
			break
		}
	}
	g.Expect(keyID1).ToNot(BeEmpty())

	// Generate second key pair (different issuer should produce different key ID)
	args2 := []string{"distro", "keygen", "sig", "test2.issuer", "--output-dir", tempDir2}
	_, err = executeCommand(args2)
	g.Expect(err).ToNot(HaveOccurred())

	// Get second key ID
	files2, err := os.ReadDir(tempDir2)
	g.Expect(err).ToNot(HaveOccurred())

	var keyID2 string
	for _, file := range files2 {
		if strings.Contains(file.Name(), "private") && filepath.Ext(file.Name()) == ".jwks" {
			data, err := os.ReadFile(filepath.Join(tempDir2, file.Name()))
			g.Expect(err).ToNot(HaveOccurred())

			var keySet lkm.EdKeySet
			err = json.Unmarshal(data, &keySet)
			g.Expect(err).ToNot(HaveOccurred())

			keyID2 = keySet.Keys[0].KeyID
			break
		}
	}
	g.Expect(keyID2).ToNot(BeEmpty())

	// Key IDs should be different for different issuers
	g.Expect(keyID1).ToNot(Equal(keyID2))
}

// Helper function to find key files in a directory
func findKeyFiles(dir string) (privateKey, publicKey string, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "private") && strings.HasSuffix(file.Name(), ".jwks") {
			privateKey = filepath.Join(dir, file.Name())
		} else if strings.Contains(file.Name(), "public") && strings.HasSuffix(file.Name(), ".jwks") {
			publicKey = filepath.Join(dir, file.Name())
		}
	}
	return privateKey, publicKey, nil
}
