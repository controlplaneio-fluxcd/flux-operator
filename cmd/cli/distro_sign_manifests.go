// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroSignManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Issue a signed attestation for manifests",
	Example: `  # Sign the manifests in the current directory
  flux-operator distro sign manifests \
  --key-set=/path/to/private.jwks \
  --attestation=attestation.jwt

  # Sign by reading the private key set from env
  export FLUX_DISTRO_SIG_PRIVATE_JWKS="$(cat /secrets/12345678-private.jwks)"
  flux-operator distro sign manifests ./distro \
  --output=./distro/attestation.jwt
`,
	Args: cobra.MaximumNArgs(1),
	RunE: distroSignManifestsCmdRun,
}

type distroSignManifestsFlags struct {
	privateKeySetPath string
	outputPath        string
}

var distroSignManifestsArgs distroSignManifestsFlags

func init() {
	distroSignManifestsCmd.Flags().StringVarP(&distroSignManifestsArgs.privateKeySetPath, "key-set", "k", "",
		"path to the JWKS file containing the private key")
	distroSignManifestsCmd.Flags().StringVarP(&distroSignManifestsArgs.outputPath, "attestation", "a", "",
		"path to to the output file for the attestation (required)")
	distroSignCmd.AddCommand(distroSignManifestsCmd)
}

func distroSignManifestsCmdRun(cmd *cobra.Command, args []string) error {
	if distroSignManifestsArgs.outputPath == "" {
		return fmt.Errorf("--attestation flag is required")
	}
	srcDir := "."
	if len(args) > 0 {
		srcDir = args[0]
	}
	if err := isDir(srcDir); err != nil {
		return err
	}

	// Read the JWKS from file or environment variable
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()
	jwksData, err := loadKeySet(ctx, distroSignManifestsArgs.privateKeySetPath, distroSigPrivateKeySetEnvVar)
	if err != nil {
		return err
	}

	// Parse the JWKS data and extract the private key
	pk, err := lkm.EdPrivateKeyFromSet(jwksData)
	if err != nil {
		return err
	}

	// Exclude the signature output file from the directory hash
	exclusion := filepath.Base(distroSignManifestsArgs.outputPath)

	// Create a new attestation and compute manifests checksum
	att := lkm.NewManifestsAttestation(distroDefaultAudience)
	tokenString, files, err := att.Sign(pk, srcDir, []string{exclusion})
	if err != nil {
		return fmt.Errorf("failed to sign manifests: %w", err)
	}

	// Print files included in the attestation
	rootCmd.Println("processing files:")
	for _, file := range files {
		rootCmd.Println(" ", file)
	}

	rootCmd.Println(fmt.Sprintf("✔ checksum: %s", att.GetChecksum()))

	// Write the signed JWT token to the output file
	err = os.WriteFile(distroSignManifestsArgs.outputPath, []byte(tokenString), 0644)
	if err != nil {
		return fmt.Errorf("failed to write attestation to file: %w", err)
	}

	rootCmd.Println(fmt.Sprintf("✔ attestation written to: %s", distroSignManifestsArgs.outputPath))

	return nil
}
