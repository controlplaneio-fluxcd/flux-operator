// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"hash/adler32"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroSignLicenseKeyCmd = &cobra.Command{
	Use:     "license-key",
	Aliases: []string{"lk"},
	Short:   "Issue a signed license key",
	Example: `  # Generate a license key that expires in one year
  cat /secrets/12345678-private.jwks | flux-operator distro sign license-key \
  --customer="Company Name LLC" \
  --duration=365 \
  --key-set=/dev/stdin \
  --output=license.jwt

  # Sign by reading the private key set from env
  export FLUX_DISTRO_PRIVATE_KEY_SET="$(cat /secrets/12345678-private.jwks)"
  flux-operator distro sign license-key \
  --customer="Company Name INC" \
  --duration=30 > demo-license.jwt
`,
	RunE: distroSignLicenseKeyCmdRun,
}

type distroSignLicenseKeyFlags struct {
	customer          string
	duration          int
	privateKeySetPath string
	outputPath        string
}

var distroSignLicenseKeyArgs distroSignLicenseKeyFlags

func init() {
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.customer, "customer", "c", "",
		"organization name for the license (required)")
	distroSignLicenseKeyCmd.Flags().IntVarP(&distroSignLicenseKeyArgs.duration, "duration", "d", 0,
		"license duration in days (required)")
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.privateKeySetPath, "key-set", "k", "",
		"path to the private key set file or /dev/stdin")
	distroSignLicenseKeyCmd.Flags().StringVarP(&distroSignLicenseKeyArgs.outputPath, "output", "o", "",
		"path to output file for the license key (defaults to stdout)")
	distroSignCmd.AddCommand(distroSignLicenseKeyCmd)
}

func distroSignLicenseKeyCmdRun(cmd *cobra.Command, args []string) error {
	if distroSignLicenseKeyArgs.customer == "" {
		return fmt.Errorf("--customer flag is required")
	}
	if distroSignLicenseKeyArgs.duration == 0 {
		return fmt.Errorf("--duration flag is required")
	}

	// Convert days to duration
	duration := time.Duration(distroSignLicenseKeyArgs.duration) * 24 * time.Hour
	if distroSignLicenseKeyArgs.duration < 0 {
		rootCmd.Println("✗ warning: negative duration will result in an expired license key")
	}

	// Read the JWKS from file or environment variable
	var jwksData []byte
	var err error
	if distroSignLicenseKeyArgs.privateKeySetPath != "" {
		// Load from file or /dev/stdin
		jwksData, err = os.ReadFile(distroSignLicenseKeyArgs.privateKeySetPath)
		if err != nil {
			return err
		}
	} else if keyData := os.Getenv(distroPrivateKeySetEnvVar); keyData != "" {
		// Load from environment variable
		jwksData = []byte(keyData)
	} else {
		return fmt.Errorf("JWKS set must be specified with --key-set flag or %s environment variable",
			distroPrivateKeySetEnvVar)
	}

	// Parse the JWKS data and extract the private key
	pk, err := lkm.EdPrivateKeyFromSet(jwksData)
	if err != nil {
		return err
	}

	// Generate the subject for the license key
	checksum := adler32.Checksum([]byte(distroSignLicenseKeyArgs.customer))
	subject := fmt.Sprintf("customer-%08x", checksum)

	// Generate the license
	now := time.Now()
	lic, err := lkm.NewLicense(lkm.LicenseKey{
		ID:           uuid.NewString(),
		Issuer:       pk.Issuer,
		Subject:      subject,
		Audience:     "flux-operator",
		Expiry:       now.Add(duration).Unix(),
		IssuedAt:     now.Unix(),
		Capabilities: nil,
	})
	if err != nil {
		return fmt.Errorf("failed to create license key: %w", err)
	}

	// Generate the license key as a signed JWT token
	token, err := lic.Sign(pk)
	if err != nil {
		return fmt.Errorf("failed to sign license key: %w", err)
	}

	// Write the signed JWT token to the output file or stdout
	if distroSignLicenseKeyArgs.outputPath != "" {
		err = os.WriteFile(distroSignLicenseKeyArgs.outputPath, []byte(token), 0644)
		if err != nil {
			return fmt.Errorf("failed to write license key to file: %w", err)
		}
		rootCmd.Println(fmt.Sprintf("✔ license key written to: %s", distroSignLicenseKeyArgs.outputPath))
	} else {
		rootCmd.Println(token)
	}

	return nil
}
