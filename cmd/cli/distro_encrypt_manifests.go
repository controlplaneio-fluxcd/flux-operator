// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroEncryptManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Encrypt manifests in a directory",
	Example: `  # Zip and encrypt the manifests in the current dir ignoring hidden files
  flux-operator distro encrypt manifests \
  --key-set=/path/to/enc-public.jwks \
  --ignore=".*,*.jwe,*private.jwks" \
  --output=manifests.zip.jwe

  # Zip and encrypt a directory using a specific public key ID
  flux-operator distro encrypt manifests ./distro \
  --key-set=/path/to/enc-public.jwks \
  --key-id=12345678-1234-1234-1234-123456789abc \
  --output=distro.zip.jwe
`,
	Args: cobra.MaximumNArgs(1),
	RunE: distroEncryptManifestsCmdRun,
}

type distroEncryptManifestsFlags struct {
	keySetPath string
	keyID      string
	outputPath string
	ignore     []string
}

var distroEncryptManifestsArgs distroEncryptManifestsFlags

func init() {
	distroEncryptManifestsCmd.Flags().StringVarP(&distroEncryptManifestsArgs.keySetPath, "key-set", "k", "",
		"path to public key set JWKS file or set the environment variable "+distroEncPublicKeySetEnvVar)
	distroEncryptManifestsCmd.Flags().StringVar(&distroEncryptManifestsArgs.keyID, "key-id", "",
		"specific key ID to use from the key set (optional, uses first suitable key if not specified)")
	distroEncryptManifestsCmd.Flags().StringVarP(&distroEncryptManifestsArgs.outputPath, "output", "o", "",
		"path to output file (required)")
	distroEncryptManifestsCmd.Flags().StringSliceVar(&distroEncryptManifestsArgs.ignore, "ignore", nil,
		"comma-separated list of glob patterns such as '.git/,*.log' (defaults to ignore *.jws)")

	distroEncryptCmd.AddCommand(distroEncryptManifestsCmd)
}

func distroEncryptManifestsCmdRun(cmd *cobra.Command, args []string) error {
	if distroEncryptManifestsArgs.outputPath == "" {
		return fmt.Errorf("--output flag is required")
	}
	srcDir := "."
	if len(args) > 0 {
		srcDir = args[0]
	}
	if err := isDir(srcDir); err != nil {
		return err
	}

	// Ensure ignore patterns are set to ignore JWS files by default
	if len(distroEncryptManifestsArgs.ignore) == 0 {
		distroEncryptManifestsArgs.ignore = []string{"*.jws"}
	}

	// Load public key set
	jwksData, err := loadKeySet(distroEncryptManifestsArgs.keySetPath, distroEncPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	var publicKeySet jose.JSONWebKeySet
	err = json.Unmarshal(jwksData, &publicKeySet)
	if err != nil {
		return fmt.Errorf("failed to parse public key set: %w", err)
	}

	// Create zip archive from directory
	zipData, fileCount, err := createZipArchive(srcDir, distroEncryptManifestsArgs.ignore)
	if err != nil {
		return fmt.Errorf("failed to create zip archive: %w", err)
	}

	// Encrypt the zip data
	jweToken, err := lkm.EncryptTokenWithKeySet(zipData, &publicKeySet, distroEncryptManifestsArgs.keyID)
	if err != nil {
		return fmt.Errorf("failed to encrypt archive: %w", err)
	}

	// Write encrypted data to output file
	err = os.WriteFile(distroEncryptManifestsArgs.outputPath, []byte(jweToken), 0644)
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	rootCmd.Printf("✔ encrypted %d files to: %s\n", fileCount, distroEncryptManifestsArgs.outputPath)
	return nil
}

// createZipArchive creates a zip archive from the specified directory,
// excluding files that match any of the ignore patterns.
// Returns the zip data as bytes, file count, and any error.
func createZipArchive(srcDir string, ignore []string) ([]byte, int, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	fileCount := 0

	// Open the source directory as a root for secure operations
	rootDir, err := os.OpenRoot(srcDir)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open source directory: %w", err)
	}
	defer func() { _ = rootDir.Close() }()

	rootCmd.Println("archiving files:")
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get relative path from source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Check if file or directory matches any ignore pattern
		for _, pattern := range ignore {
			if matchesPattern(relPath, pattern) {
				if d.IsDir() {
					return filepath.SkipDir // Skip entire directory and its contents
				}
				return nil // Skip this file
			}
		}

		if d.IsDir() {
			// Create directory entry in zip
			dirPath := filepath.ToSlash(relPath) + "/"
			_, err := zipWriter.Create(dirPath)
			if err != nil {
				return err
			}
		} else {
			// Skip non-regular files (e.g., symlinks, devices)
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			// Add file to zip archive
			zipFile, err := zipWriter.Create(filepath.ToSlash(relPath))
			if err != nil {
				return err
			}

			// Use rootDir to safely open the file
			fileReader, err := rootDir.Open(relPath)
			if err != nil {
				return err
			}

			_, copyErr := io.Copy(zipFile, fileReader)
			closeErr := fileReader.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}

			rootCmd.Println(" ", filepath.ToSlash(relPath))
			fileCount++
		}
		return nil
	})

	if err != nil {
		if closeErr := zipWriter.Close(); closeErr != nil {
			return nil, 0, fmt.Errorf("walk error: %w, close error: %w", err, closeErr)
		}
		return nil, 0, err
	}

	err = zipWriter.Close()
	if err != nil {
		return nil, 0, err
	}

	return buf.Bytes(), fileCount, nil
}

// matchesPattern checks if a file path matches a given pattern.
// Supports glob patterns (*.ext) and directory patterns (dir/).
func matchesPattern(filePath, pattern string) bool {
	// Handle directory patterns (ending with /) - matches directory and its contents
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dir+"/") || filePath == dir
	}

	// Try glob pattern matching on the base filename (e.g., *.go matches file.go)
	if matched, err := filepath.Match(pattern, filepath.Base(filePath)); err == nil && matched {
		return true
	}

	// Try glob pattern matching on the full path (e.g., docs/*.md matches docs/readme.md)
	if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
		return true
	}

	return false
}
