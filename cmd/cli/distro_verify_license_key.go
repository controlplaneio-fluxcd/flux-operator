// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
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
	publicKeySetPath string
}

var distroVerifyLicenseKeyArgs distroVerifyLicenseKeyFlags

func init() {
	distroVerifyLicenseKeyCmd.Flags().StringVarP(&distroVerifyLicenseKeyArgs.publicKeySetPath, "key-set", "k", "",
		"path to the public key set file or /dev/stdin")
	distroVerifyCmd.AddCommand(distroVerifyLicenseKeyCmd)
}

func distroVerifyLicenseKeyCmdRun(cmd *cobra.Command, args []string) error {
	licenseFile := args[0]

	// Read the JWT license key from file
	licenseData, err := os.ReadFile(licenseFile)
	if err != nil {
		return fmt.Errorf("failed to read license file: %w", err)
	}

	// Parse the JWT license key to extract the key ID
	signedObject, err := jose.ParseSigned(string(licenseData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return fmt.Errorf("failed to parse signed license key: %w", err)
	}

	// Extract key ID from the protected header
	if len(signedObject.Signatures) == 0 {
		return fmt.Errorf("no signatures found in license key")
	}
	kid := signedObject.Signatures[0].Protected.KeyID
	if kid == "" {
		return fmt.Errorf("no key ID found in license key protected header")
	}

	// Load the JWKS and find the matching key
	var jwksData []byte
	if distroVerifyLicenseKeyArgs.publicKeySetPath != "" {
		// Load from file or /dev/stdin
		jwksData, err = os.ReadFile(distroVerifyLicenseKeyArgs.publicKeySetPath)
		if err != nil {
			return err
		}
	} else if keyData := os.Getenv(distroPublicKeySetEnvVar); keyData != "" {
		// Load from environment variable
		jwksData = []byte(keyData)
	} else {
		return fmt.Errorf("JWKS must be specified with --key-set flag or %s environment variable",
			distroPublicKeySetEnvVar)
	}

	// Extract the public key for the specific key ID
	publicKey, err := parsePublicKeySet(jwksData, kid)
	if err != nil {
		return err
	}

	// Verify the signature with the public key
	payload, err := signedObject.Verify(publicKey)
	if err != nil {
		return fmt.Errorf("failed to verify license key signature: %w", err)
	}

	// Parse the claims from the payload
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("failed to parse license key claims: %w", err)
	}

	// Validate required claims
	if err := validateLicenseKeyClaims(claims); err != nil {
		return err
	}

	// Get issuer from the JWT claims
	issuer, _ := claims["iss"].(string)

	// Get expiration time
	exp, _ := claims["exp"].(float64)
	expirationTime := time.Unix(int64(exp), 0)

	// Display license information and check validity
	rootCmd.Println(fmt.Sprintf("✔ license key is issued by %s", issuer))
	if expirationTime.Before(time.Now()) {
		return fmt.Errorf("license key has expired on %s", expirationTime.Format(time.RFC3339))
	}

	rootCmd.Println(fmt.Sprintf("✔ license key is valid until %s", expirationTime.Format(time.RFC3339)))
	return nil
}

// validateLicenseKeyClaims validates the JWT claims for a license key
func validateLicenseKeyClaims(claims map[string]any) error {
	// Check issuer
	issuer, ok := claims["iss"].(string)
	if !ok {
		return fmt.Errorf("issuer not found in license key claims")
	}
	if issuer == "" {
		return fmt.Errorf("issuer cannot be empty in license key claims")
	}

	// Check subject (customer id)
	subject, ok := claims["sub"].(string)
	if !ok {
		return fmt.Errorf("subject not found in license key claims")
	}
	if subject == "" {
		return fmt.Errorf("subject cannot be empty in license key claims")
	}

	// Check audience
	audience, ok := claims["aud"].(string)
	if !ok {
		return fmt.Errorf("audience not found in license key claims")
	}
	if audience == "" {
		return fmt.Errorf("audience cannot be empty in license key claims")
	}

	// Check expiration time
	exp, ok := claims["exp"].(float64)
	if !ok {
		return fmt.Errorf("expiration time not found in license key claims")
	}
	if exp <= 0 {
		return fmt.Errorf("invalid expiration time in license key claims")
	}

	// Check issued at time
	iat, ok := claims["iat"].(float64)
	if !ok {
		return fmt.Errorf("issued at time not found in license key claims")
	}
	if iat <= 0 {
		return fmt.Errorf("invalid issued at time in license key claims")
	}

	// Validate issued at time is not in the future (allow 1 minute clock skew)
	issuedAtTime := time.Unix(int64(iat), 0)
	if issuedAtTime.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("license key issued at time is in the future: %s", issuedAtTime.Format(time.RFC3339))
	}

	return nil
}
