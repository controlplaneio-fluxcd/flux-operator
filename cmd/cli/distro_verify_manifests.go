// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
	"golang.org/x/mod/sumdb/dirhash"
)

var distroVerifyManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Verify signed manifests",
	Example: `  # Verify the signed manifests in the current directory
  cat /secrets/12345678-public.jwks | flux-operator distro verify manifests \
  --key-set=/dev/stdin \
  --signature=signature.jwt

  # Verify by reading the public key set from env
  export FLUX_DISTRO_PUBLIC_KEY_SET="$(cat /secrets/12345678-public.jwks)"
  flux-operator distro verify manifests ./distro \
  --signature=signature.jwt
`,
	Args: cobra.MaximumNArgs(1),
	RunE: distroVerifyManifestsCmdRun,
}

type distroVerifyManifestsFlags struct {
	publicKeySetPath string
	signaturePath    string
}

var distroVerifyManifestsArgs distroVerifyManifestsFlags

func init() {
	distroVerifyManifestsCmd.Flags().StringVarP(&distroVerifyManifestsArgs.publicKeySetPath, "key-set", "k", "",
		"path to the public key set file or /dev/stdin (required)")
	distroVerifyManifestsCmd.Flags().StringVarP(&distroVerifyManifestsArgs.signaturePath, "signature", "s", "",
		"path to the signature file (required)")
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

	// Read the JWT signature from file to extract the key ID
	signatureData, err := os.ReadFile(distroVerifyManifestsArgs.signaturePath)
	if err != nil {
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	// Parse the JWT signature to extract the key ID
	signedObject, err := jose.ParseSigned(string(signatureData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return fmt.Errorf("failed to parse signed token: %w", err)
	}

	// Extract key ID from the protected header
	if len(signedObject.Signatures) == 0 {
		return fmt.Errorf("no signatures found in JWT")
	}
	kid := signedObject.Signatures[0].Protected.KeyID
	if kid == "" {
		return fmt.Errorf("no key ID found in JWT protected header")
	}

	// Load the JWKS and find the matching key
	var jwksData []byte
	if distroVerifyManifestsArgs.publicKeySetPath != "" {
		// Load from file or /dev/stdin
		jwksData, err = os.ReadFile(distroVerifyManifestsArgs.publicKeySetPath)
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
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	// Parse the claims from the payload
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("failed to parse claims: %w", err)
	}

	// Get the expected checksum from the JWT subject
	expectedChecksum, ok := claims["sub"].(string)
	if !ok {
		return fmt.Errorf("checksum not found in claims")
	}

	// Get issuer from the JWT claims
	issuer, ok := claims["iss"].(string)
	if !ok {
		return fmt.Errorf("issuer not found in claims")
	}

	// Exclude the signature file from the directory hash
	exclusion := filepath.Base(distroVerifyManifestsArgs.signaturePath)

	// Print all files that will be processed
	files, err := dirFiles(srcDir, "", exclusion)
	if err != nil {
		return err
	}
	rootCmd.Println("processing files:")
	for _, file := range files {
		rootCmd.Println(" ", file)
	}

	// Calculate the current directory hash excluding the signature file
	actualChecksum, err := hashDir(srcDir, "", exclusion, dirhash.DefaultHash)
	if err != nil {
		return err
	}

	// Compare checksums
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum verification failed: calculated %s, expected %s", actualChecksum, expectedChecksum)
	}

	rootCmd.Println(fmt.Sprintf("✔ signature issued by %s is valid", issuer))

	return nil
}

// parsePublicKeySet extracts an Ed25519 public key from JWKS data using the specified key ID
func parsePublicKeySet(jwksData []byte, kid string) (ed25519.PublicKey, error) {
	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(jwksData, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %w", err)
	}

	// Find the key with matching key ID
	for _, key := range jwks.Keys {
		if key.KeyID == kid {
			// Validate the key properties
			if key.Algorithm != string(jose.EdDSA) {
				return nil, fmt.Errorf("key %s has unsupported algorithm %s, expected %s", kid, key.Algorithm, jose.EdDSA)
			}
			if key.Use != "sig" {
				return nil, fmt.Errorf("key %s has unsupported use %s, expected 'sig'", kid, key.Use)
			}

			// Extract the Ed25519 public key
			publicKey, ok := key.Key.(ed25519.PublicKey)
			if !ok {
				return nil, fmt.Errorf("key %s is not an Ed25519 public key", kid)
			}

			return publicKey, nil
		}
	}

	return nil, fmt.Errorf("key with ID %s not found in JWKS", kid)
}
