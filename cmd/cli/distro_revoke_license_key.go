// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroRevokeLicenseKeyCmd = &cobra.Command{
	Use:     "license-key [LICENSE_FILE]",
	Aliases: []string{"lk"},
	Short:   "Revoke a signed license key",
	Example: `  # Revoke a license key and save to banlist file
  flux-operator distro revoke license-key license1.jwt \
  --output=banlist.rks

  # Revoke a license key and merge with existing banlist
  flux-operator distro revoke license-key license2.jwt \
  --output=banlist.rks
`,
	Args: cobra.ExactArgs(1),
	RunE: distroRevokeLicenseKeyCmdRun,
}

type distroRevokeLicenseKeyFlags struct {
	outputPath string
}

var distroRevokeLicenseKeyArgs distroRevokeLicenseKeyFlags

func init() {
	distroRevokeLicenseKeyCmd.Flags().StringVarP(&distroRevokeLicenseKeyArgs.outputPath, "output", "o", "",
		"path to output revocation set file (required)")
	distroRevokeCmd.AddCommand(distroRevokeLicenseKeyCmd)
}

func distroRevokeLicenseKeyCmdRun(cmd *cobra.Command, args []string) error {
	licenseFile := args[0]

	if distroRevokeLicenseKeyArgs.outputPath == "" {
		return fmt.Errorf("--output flag is required")
	}

	// Read the signed JWT from file
	jwtData, err := os.ReadFile(licenseFile)
	if err != nil {
		return fmt.Errorf("failed to read the license key: %w", err)
	}

	// Extract the license ID from the JWT token payload (without signature verification)
	licenseID, issuer, err := extractLicenseIDFromToken(jwtData)
	if err != nil {
		return fmt.Errorf("failed to extract license ID from token: %w", err)
	}

	// Create or load existing revocation set
	rks := lkm.NewRevocationKeySet(issuer)

	// Add the license key ID to revocation set
	if err := rks.AddKey(licenseID); err != nil {
		return fmt.Errorf("failed to add license key to revocation set: %w", err)
	}

	// Write the revocation set to file (will merge if file exists)
	if err := rks.WriteFile(distroRevokeLicenseKeyArgs.outputPath); err != nil {
		return fmt.Errorf("failed to write revocation set: %w", err)
	}

	rootCmd.Println(fmt.Sprintf("✔ license key %s revoked and saved to: %s", licenseID, distroRevokeLicenseKeyArgs.outputPath))
	return nil
}

// extractLicenseIDFromToken extracts the license ID (jti claim) from a JWT token
// without verifying the signature. This is used for revocation purposes.
func extractLicenseIDFromToken(jwtData []byte) (string, string, error) {
	// Parse the JWT using go-jose to get the payload without verification
	jws, err := jose.ParseSigned(string(jwtData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return "", "", fmt.Errorf("invalid JWT format: %w", err)
	}

	// Get the payload without verification (this is safe for revocation purposes)
	payload := jws.UnsafePayloadWithoutVerification()

	// Parse the payload as LicenseKey to extract the jti claim
	var lk lkm.LicenseKey
	if err := json.Unmarshal(payload, &lk); err != nil {
		return "", "", fmt.Errorf("failed to parse license claims: %w", err)
	}

	// Validate that the license ID exists
	if lk.ID == "" {
		return "", "", fmt.Errorf("license token missing jti (license ID) claim")
	}

	if lk.Issuer == "" {
		return "", "", fmt.Errorf("license token missing issuer claim")
	}

	return lk.ID, lk.Issuer, nil
}
