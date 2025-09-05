// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroDecryptManifestsCmd = &cobra.Command{
	Use:   "manifests [JWE_FILE]",
	Short: "Decrypt and extract manifests from an encrypted archive",
	Example: `  # Decrypt manifests to current directory
  flux-operator distro decrypt manifests manifests.zip.jwe \
  --key-set=/path/to/enc-private.jwks

  # Decrypt to specific directory and overwrite existing files
  export FLUX_DISTRO_ENC_PRIVATE_JWKS="$(cat /path/to/private.jwks)"
  flux-operator distro decrypt manifests distro.zip.jwe \
  --overwrite \
  --output-dir=./distro
`,
	Args: cobra.ExactArgs(1),
	RunE: distroDecryptManifestsCmdRun,
}

type distroDecryptManifestsFlags struct {
	keySetPath string
	outputPath string
	overwrite  bool
}

var distroDecryptManifestsArgs distroDecryptManifestsFlags

func init() {
	distroDecryptManifestsCmd.Flags().StringVarP(&distroDecryptManifestsArgs.keySetPath, "key-set", "k", "",
		"path to JWKS file containing the private key")
	distroDecryptManifestsCmd.Flags().StringVarP(&distroDecryptManifestsArgs.outputPath, "output-dir", "o", ".",
		"path to output directory (defaults to current directory)")
	distroDecryptManifestsCmd.Flags().BoolVar(&distroDecryptManifestsArgs.overwrite, "overwrite", false,
		"overwrite existing files")

	distroDecryptCmd.AddCommand(distroDecryptManifestsCmd)
}

func distroDecryptManifestsCmdRun(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// Validate input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file %s does not exist", inputFile)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(distroDecryptManifestsArgs.outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load private key set
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()
	jwksData, err := loadKeySet(ctx, distroDecryptManifestsArgs.keySetPath, distroEncPrivateKeySetEnvVar)
	if err != nil {
		return err
	}

	var privateKeySet jose.JSONWebKeySet
	err = json.Unmarshal(jwksData, &privateKeySet)
	if err != nil {
		return fmt.Errorf("failed to parse private key set: %w", err)
	}

	// Read and decrypt the JWE file
	jweData, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	decryptedData, err := lkm.DecryptTokenWithKeySet(jweData, &privateKeySet)
	if err != nil {
		return fmt.Errorf("failed to decrypt archive: %w", err)
	}

	// Extract zip archive
	extractedFiles, err := extractZipArchive(decryptedData, distroDecryptManifestsArgs.outputPath, distroDecryptManifestsArgs.overwrite)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Print extracted files
	rootCmd.Println("extracted files:")
	for _, file := range extractedFiles {
		rootCmd.Println(" ", file)
	}

	rootCmd.Printf("✔ extracted %d files to: %s\n", len(extractedFiles), distroDecryptManifestsArgs.outputPath)
	return nil
}

// extractZipArchive extracts a zip archive from bytes to the specified directory.
// Returns the list of extracted files.
func extractZipArchive(zipData []byte, destDir string, overwrite bool) ([]string, error) {
	reader := bytes.NewReader(zipData)
	zipReader, err := zip.NewReader(reader, int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip archive: %w", err)
	}

	var extractedFiles []string

	// Open the destination directory as a root to prevent path traversal
	rootDir, err := os.OpenRoot(destDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open destination directory: %w", err)
	}
	defer func() { _ = rootDir.Close() }()

	for _, file := range zipReader.File {
		// Check for file conflicts
		if !overwrite {
			if _, err := rootDir.Stat(file.Name); err == nil {
				return nil, fmt.Errorf("file %s already exists (use --overwrite to replace)", file.Name)
			}
		}

		if file.FileInfo().IsDir() {
			// Create directory
			err := rootDir.MkdirAll(file.Name, 0755)
			if err != nil {
				return nil, fmt.Errorf("failed to create directory %s: %w", file.Name, err)
			}
			// Don't add directories to the extracted files list for counting
		} else {
			// Extract file
			err := extractFileSecure(rootDir, file)
			if err != nil {
				return nil, fmt.Errorf("failed to extract file %s: %w", file.Name, err)
			}
			extractedFiles = append(extractedFiles, file.Name)
		}
	}

	return extractedFiles, nil
}

// extractFileSecure extracts a single file from the zip archive using os.Root for security.
func extractFileSecure(rootDir *os.Root, file *zip.File) error {
	// Ensure the destination directory exists
	destDir := filepath.Dir(file.Name)
	if destDir != "." && destDir != "" {
		if err := rootDir.MkdirAll(destDir, 0755); err != nil {
			return err
		}
	}

	// Open the file in the zip archive
	reader, err := file.Open()
	if err != nil {
		return err
	}

	// Create the destination file using the secure root
	writer, err := rootDir.OpenFile(file.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.FileInfo().Mode())
	if err != nil {
		_ = reader.Close()
		return err
	}

	// Copy the file contents
	_, copyErr := io.Copy(writer, reader)

	// Close both files and return any error
	readerErr := reader.Close()
	writerErr := writer.Close()

	if copyErr != nil {
		return copyErr
	}
	if readerErr != nil {
		return readerErr
	}
	return writerErr
}
