// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroVerifyManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Verify the attestation of manifests",
	Example: `  # Verify the attestation of manifests in the current directory
  cat /secrets/12345678-public.jwks | flux-operator distro verify manifests \
  --key-set=/dev/stdin \
  --attestation=attestation.jwt

  # Verify by reading the public key set from env
  export FLUX_DISTRO_PUBLIC_KEY_SET="$(cat /secrets/12345678-public.jwks)"
  flux-operator distro verify manifests ./distro \
  --attestation=attestation.jwt
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
		"path to the public key set file or /dev/stdin (required)")
	distroVerifyManifestsCmd.Flags().StringVarP(&distroVerifyManifestsArgs.attestationPath, "attestation", "a", "",
		"path to the attestation file (required)")
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

	// Read the signed JWT from file
	jwtData, err := os.ReadFile(distroVerifyManifestsArgs.attestationPath)
	if err != nil {
		return fmt.Errorf("failed to read the attestion file: %w", err)
	}

	// Extract the public key ID from the JWT header
	kid, err := lkm.GetKeyIDFromToken(jwtData)
	if err != nil {
		return lkm.InvalidAttestationError(err)
	}

	// Load the JWKS and find the matching key
	jwksData, err := loadKeySet(distroVerifyManifestsArgs.publicKeySetPath, distroPublicKeySetEnvVar)
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
		return err
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
