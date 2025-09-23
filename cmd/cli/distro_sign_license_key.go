// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroSignLicenseKeyCmd = &cobra.Command{
	Use:     "license-key",
	Aliases: []string{"lk"},
	Short:   "Issue a signed license key",
	Example: `  # Generate a license key that expires in one year
  flux-operator distro sign license-key \
  --key-set=/path/to/private.jwks \
  --customer="Company Name LLC" \
  --duration=365 \
  --output=license.jwt

  # Generate a license key that grants access to specific capabilities
  export FLUX_DISTRO_SIG_PRIVATE_JWKS="$(cat /path/to/private.jwks)"
  flux-operator distro sign license-key \
  --customer="Company Name INC" \
  --capabilities="feature1,feature2" \
  --duration=30 \
  --output=license.jwt
`,
	RunE: distroSignLicenseKeyCmdRun,
}

type distroSignLicenseKeyFlags struct {
	customer          string
	capabilities      []string
	duration          int
	privateKeySetPath string
	outputPath        string
}

var distroSignLicenseKeyArgs distroSignLicenseKeyFlags

func init() {
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.customer, "customer", "c", "",
		"organization name for the license (required)")
	distroSignLicenseKeyCmd.Flags().StringSliceVar(&distroSignLicenseKeyArgs.capabilities, "capabilities", nil,
		"license capabilities (optional)")
	distroSignLicenseKeyCmd.Flags().IntVarP(&distroSignLicenseKeyArgs.duration, "duration", "d", 0,
		"license duration in days (required)")
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.privateKeySetPath, "key-set", "k", "",
		"path to the JWKS file containing the private key")
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.outputPath, "output", "o", "",
		"path to the output file for the license key (required)")
	distroSignCmd.AddCommand(distroSignLicenseKeyCmd)
}

func distroSignLicenseKeyCmdRun(cmd *cobra.Command, args []string) error {
	if distroSignLicenseKeyArgs.customer == "" {
		return fmt.Errorf("--customer flag is required")
	}
	if distroSignLicenseKeyArgs.duration == 0 {
		return fmt.Errorf("--duration flag is required")
	}
	if distroSignLicenseKeyArgs.outputPath == "" {
		return fmt.Errorf("--output flag is required")
	}
	if distroSignLicenseKeyArgs.duration < 0 {
		rootCmd.Println("✗ warning: negative duration will result in an expired license key")
	}

	// Read the JWKS from file or environment variable
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()
	jwksData, err := loadKeySet(ctx, distroSignLicenseKeyArgs.privateKeySetPath, distroSigPrivateKeySetEnvVar)
	if err != nil {
		return err
	}

	// Parse the JWKS data and extract the private key
	pk, err := lkm.EdPrivateKeyFromSet(jwksData)
	if err != nil {
		return err
	}

	// Generate the license
	lic, err := lkm.NewLicense(
		pk.Issuer,
		distroSignLicenseKeyArgs.customer,
		distroDefaultAudience,
		time.Duration(distroSignLicenseKeyArgs.duration)*24*time.Hour,
		distroSignLicenseKeyArgs.capabilities,
	)
	if err != nil {
		return fmt.Errorf("failed to create license: %w", err)
	}

	// Generate the license key as a signed JWT token
	token, err := lic.Sign(pk)
	if err != nil {
		return fmt.Errorf("failed to sign license key: %w", err)
	}

	// Write the signed JWT token to the output file
	err = os.WriteFile(distroSignLicenseKeyArgs.outputPath, []byte(token), 0644)
	if err != nil {
		return fmt.Errorf("failed to write license key to file: %w", err)
	}
	rootCmd.Println(fmt.Sprintf("✔ license key written to: %s", distroSignLicenseKeyArgs.outputPath))

	return nil
}
