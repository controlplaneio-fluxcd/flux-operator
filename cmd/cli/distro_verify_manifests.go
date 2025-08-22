// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroVerifyManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Verify the attestation of manifests",
	Example: `  # Verify the attestation of manifests in the current directory
  flux-operator distro verify manifests \
  --key-set=https://example.com/jwks.json \
  --attestation=https://example.com/attestation.jwt

  # Verify by reading the public key set from env
  export FLUX_DISTRO_SIG_PUBLIC_JWKS="$(cat /path/to/public.jwks)"
  flux-operator distro verify manifests ./distro \
  -a /path/to/attestation.jwt
`,
	Args: cobra.MaximumNArgs(1),
	RunE: distroVerifyManifestsCmdRun,
}

type distroVerifyManifestsFlags struct {
	publicKeySetPath string
	attestationPath  string
}

var distroVerifyManifestsArgs distroVerifyManifestsFlags

func init() {
	distroVerifyManifestsCmd.Flags().StringVarP(&distroVerifyManifestsArgs.publicKeySetPath, "key-set", "k", "",
		"path to the JWKS file containing the public keys or HTTPS URL")
	distroVerifyManifestsCmd.Flags().StringVarP(&distroVerifyManifestsArgs.attestationPath, "attestation", "a", "",
		"path to the attestation file or HTTPS URL (required)")
	distroVerifyCmd.AddCommand(distroVerifyManifestsCmd)
}

func distroVerifyManifestsCmdRun(cmd *cobra.Command, args []string) error {
	srcDir := "."
	if len(args) > 0 {
		srcDir = args[0]
	}
	if err := isDir(srcDir); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Read the signed JWT from HTTP URL or file
	jwtData, err := loadAttestation(ctx, distroVerifyManifestsArgs.attestationPath)
	if err != nil {
		return err
	}

	// Extract the public key ID from the JWT header
	kid, err := lkm.GetKeyIDFromToken(jwtData)
	if err != nil {
		return lkm.InvalidAttestationError(err)
	}

	// Load the JWKS and find the matching key
	jwksData, err := loadKeySet(ctx, distroVerifyManifestsArgs.publicKeySetPath, distroSigPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	// Extract the public key for the specific key ID
	pk, err := lkm.EdPublicKeyFromSet(jwksData, kid)
	if err != nil {
		return err
	}

	// Exclude the signature file from the directory hash
	exclusion := filepath.Base(distroVerifyManifestsArgs.attestationPath)

	// Create an attestation verifier
	att := lkm.NewManifestsAttestation(distroDefaultAudience)

	// Verify the JWT signature, calculate the expected checksum, and list files
	files, err := att.Verify(jwtData, pk, srcDir, []string{exclusion})
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print files included in the checksum
	rootCmd.Println("processing files:")
	for _, file := range files {
		rootCmd.Println(" ", file)
	}

	// Print the checksum and issuer
	rootCmd.Println(fmt.Sprintf("✔ attestation issued by %s at %s is valid", att.GetIssuer(), att.GetIssuedAt()))
	rootCmd.Println(fmt.Sprintf("✔ verified checksum: %s", att.GetChecksum()))

	return nil
}
