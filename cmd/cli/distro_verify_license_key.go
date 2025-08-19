// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroVerifyLicenseKeyCmd = &cobra.Command{
	Use:     "license-key [LICENSE_FILE]",
	Aliases: []string{"lk"},
	Short:   "Verify a signed license key",
	Example: `  # Verify a license key with public key set from file
  cat /secrets/12345678-public.jwks | flux-operator distro verify license-key license.jwt \
  --key-set=/dev/stdin

  # Verify by reading the public key set from env
  export FLUX_DISTRO_PUBLIC_KEY_SET="$(cat /secrets/12345678-public.jwks)"
  flux-operator distro verify license-key ./license.jwt
`,
	Args: cobra.ExactArgs(1),
	RunE: distroVerifyLicenseKeyCmdRun,
}

type distroVerifyLicenseKeyFlags struct {
	publicKeySetPath  string
	revokedKeySetPath string
}

var distroVerifyLicenseKeyArgs distroVerifyLicenseKeyFlags

func init() {
	distroVerifyLicenseKeyCmd.Flags().StringVarP(&distroVerifyLicenseKeyArgs.publicKeySetPath, "key-set", "k", "",
		"path to the public key set file or /dev/stdin")
	distroVerifyLicenseKeyCmd.Flags().StringVarP(&distroVerifyLicenseKeyArgs.revokedKeySetPath, "revoked-set", "r", "",
		"path to the revoked key set file (optional)")
	distroVerifyCmd.AddCommand(distroVerifyLicenseKeyCmd)
}

func distroVerifyLicenseKeyCmdRun(cmd *cobra.Command, args []string) error {
	licenseFile := args[0]

	// Read the signed JWT from file
	jwtData, err := os.ReadFile(licenseFile)
	if err != nil {
		return fmt.Errorf("failed to read the license key: %w", err)
	}

	// Extract the public key ID from the JWT header
	kid, err := lkm.GetKeyIDFromToken(jwtData)
	if err != nil {
		return lkm.InvalidLicenseKeyError(err)
	}

	// Read the JWKS data from the specified source
	jwksData, err := loadKeySet(distroVerifyLicenseKeyArgs.publicKeySetPath, distroPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	// Extract the public key for the specific ID
	pk, err := lkm.EdPublicKeyFromSet(jwksData, kid)
	if err != nil {
		return lkm.InvalidLicenseKeyError(err)
	}

	// Verify the JWT signature and extract the license information
	lic, err := lkm.GetLicenseFromToken(jwtData, pk)
	if err != nil {
		return err
	}

	// Display license key information
	rootCmd.Println(fmt.Sprintf("✔ license key was issued by %s at %s", lic.GetIssuer(), lic.GetIssuedAt()))
	if caps := lic.GetKey().Capabilities; len(caps) > 0 {
		rootCmd.Println(fmt.Sprintf("✔ license key capabilities: %s", strings.Join(caps, ", ")))
	}

	// Check if the license key is revoked
	if distroVerifyLicenseKeyArgs.revokedKeySetPath != "" {
		// Load the revoked key set from file
		rksData, err := os.ReadFile(distroVerifyLicenseKeyArgs.revokedKeySetPath)
		if err != nil {
			return fmt.Errorf("failed to read the revoked key set: %w", err)
		}

		rks, err := lkm.RevocationKeySetFromJSON(rksData)
		if err != nil {
			return fmt.Errorf("failed to parse revoked key set: %w", err)
		}

		// Check if the license key ID is in the revoked set
		if revoked, ts := rks.IsRevoked(lic); revoked {
			return fmt.Errorf("license key is revoked since %s", ts)
		}
	}

	// Check if the license key is expired
	if lic.IsExpired(time.Second) {
		return fmt.Errorf("license key has expired on %s", lic.GetExpiry())
	}
	rootCmd.Println(fmt.Sprintf("✔ license key is valid until %s", lic.GetExpiry()))
	return nil
}
