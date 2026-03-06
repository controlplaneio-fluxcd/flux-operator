// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/cli/lkm"
)

var distroVerifyLicenseKeyCmd = &cobra.Command{
	Use:     "license-key [LICENSE_FILE]",
	Aliases: []string{"lk"},
	Short:   "Verify a signed license key",
	Example: `  # Verify a license key with public key set from file
  flux-operator distro verify license-key /path/to/license.jwt \
  --key-set=https://example.com/jwks.json

  # Verify by reading the public key set from env
  export FLUX_DISTRO_SIG_PUBLIC_JWKS="$(cat /path/to/public.jwks)"
  flux-operator distro verify license-key /path/to/license.jwt
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
		"path to the JWKS file containing the public keys or HTTPS URL")
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

	// Read the JWKS data from the specified source
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()
	jwksData, err := loadKeySet(ctx, distroVerifyLicenseKeyArgs.publicKeySetPath, distroSigPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	// Read the revoked key set if specified
	var revokedKeysJSON []byte
	if distroVerifyLicenseKeyArgs.revokedKeySetPath != "" {
		revokedKeysJSON, err = os.ReadFile(distroVerifyLicenseKeyArgs.revokedKeySetPath)
		if err != nil {
			return fmt.Errorf("failed to read the revoked key set: %w", err)
		}
	}

	// Verify the license key signature, claims, expiry, and revocation
	result, err := lkm.VerifyLicenseKey(jwksData, jwtData, revokedKeysJSON)
	if err != nil {
		return err
	}

	// Display license key information
	rootCmd.Println(fmt.Sprintf("✔ license key was issued by %s at %s", result.Issuer, result.IssuedAt))
	if len(result.Capabilities) > 0 {
		rootCmd.Println(fmt.Sprintf("✔ license key capabilities: %s", strings.Join(result.Capabilities, ", ")))
	}
	rootCmd.Println(fmt.Sprintf("✔ license key is valid until %s", result.Expiry))
	return nil
}
