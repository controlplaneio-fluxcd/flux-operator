// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
	"golang.org/x/mod/sumdb/dirhash"
)

var distroSignManifestsCmd = &cobra.Command{
	Use:   "manifests [DIRECTORY]",
	Short: "Sign manifests",
	Example: `  # Sign the manifests in the current directory
  cat /secrets/12345678-private.jwks | flux-operator distro sign manifests \
  --key-set=/dev/stdin \
  --signature=signature.jwt

  # Sign by reading the private key set from env
  export FLUX_DISTRO_PRIVATE_KEY_SET="$(cat /secrets/12345678-private.jwks)"
  flux-operator distro sign manifests ./distro \
  --signature=./distro/signature.jwt
`,
	Args: cobra.MaximumNArgs(1),
	RunE: distroSignManifestsCmdRun,
}

type distroSignManifestsFlags struct {
	privateKeySetPath   string
	signatureOutputPath string
}

var distroSignManifestsArgs distroSignManifestsFlags

func init() {
	distroSignManifestsCmd.Flags().StringVarP(&distroSignManifestsArgs.privateKeySetPath, "key-set", "k", "",
		"path to the private key set file or /dev/stdin (required)")
	distroSignManifestsCmd.Flags().StringVarP(&distroSignManifestsArgs.signatureOutputPath, "signature", "s", "",
		"path to output file for the signature (required)")
	distroSignCmd.AddCommand(distroSignManifestsCmd)
}

func distroSignManifestsCmdRun(cmd *cobra.Command, args []string) error {
	if distroSignManifestsArgs.signatureOutputPath == "" {
		return fmt.Errorf("--signature flag is required")
	}
	srcDir := "."
	if len(args) > 0 {
		srcDir = args[0]
	}
	if err := isDir(srcDir); err != nil {
		return err
	}

	// Read the JWKS from file or environment variable
	var jwksData []byte
	var err error
	if distroSignManifestsArgs.privateKeySetPath != "" {
		// Load from file or /dev/stdin
		jwksData, err = os.ReadFile(distroSignManifestsArgs.privateKeySetPath)
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

	// Extract the private key and issuer from the JWKS data
	privateKey, issuer, keyID, err := parsePrivateKeySet(jwksData)
	if err != nil {
		return err
	}

	// Exclude the signature output file from the directory hash
	exclusion := filepath.Base(distroSignManifestsArgs.signatureOutputPath)

	// Print all files that will be processed
	files, err := dirFiles(srcDir, "", exclusion)
	if err != nil {
		return err
	}
	rootCmd.Println("processing files:")
	for _, file := range files {
		rootCmd.Println(" ", file)
	}

	// Calculate the directory hash excluding the signature output file
	checksum, err := hashDir(srcDir, "", exclusion, dirhash.DefaultHash)
	if err != nil {
		return err
	}
	rootCmd.Println(fmt.Sprintf("✔ checksum: %s", checksum))

	// Generate claims with issuer and checksum
	claims := map[string]any{
		"iss": issuer,
		"sub": checksum,
		"aud": "flux-operator",
		"iat": time.Now().Unix(),
	}

	// Create payload
	payload, err := json.Marshal(claims)
	if err != nil {
		return fmt.Errorf("failed to marshal claims: %w", err)
	}

	// Create signer with Ed25519 private key
	signerOpts := jose.SignerOptions{}
	signerOpts.WithType("JWT")
	signerOpts.WithHeader("kid", keyID)

	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.EdDSA,
		Key:       privateKey,
	}, &signerOpts)
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	// Sign the payload
	signedObject, err := signer.Sign(payload)
	if err != nil {
		return fmt.Errorf("failed to sign payload: %w", err)
	}

	// Get the compact serialization (JWT format)
	tokenString, err := signedObject.CompactSerialize()
	if err != nil {
		return fmt.Errorf("failed to serialize signed token: %w", err)
	}

	// Write the signed JWT token to the output file
	err = os.WriteFile(distroSignManifestsArgs.signatureOutputPath, []byte(tokenString), 0644)
	if err != nil {
		return fmt.Errorf("failed to write signature to file: %w", err)
	}

	rootCmd.Println(fmt.Sprintf("✔ signature written to: %s", distroSignManifestsArgs.signatureOutputPath))

	return nil
}

// parsePrivateKeySet parses Ed25519 private key from PrivateKeySet JSON data
func parsePrivateKeySet(keySetData []byte) (ed25519.PrivateKey, string, string, error) {
	var privateKeySet PrivateKeySet
	if err := json.Unmarshal(keySetData, &privateKeySet); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse private key set: %w", err)
	}

	if privateKeySet.Issuer == "" {
		return nil, "", "", fmt.Errorf("issuer information is missing in the private key set")
	}

	if len(privateKeySet.Keys) == 0 {
		return nil, "", "", fmt.Errorf("no keys found in private key set")
	}

	// Use the first key as specified
	firstKey := privateKeySet.Keys[0]

	if firstKey.KeyID == "" {
		return nil, "", "", fmt.Errorf("key ID is missing in the first key")
	}

	if firstKey.Algorithm != string(jose.EdDSA) {
		return nil, "", "", fmt.Errorf("first key has unsupported algorithm %s, expected %s", firstKey.Algorithm, jose.EdDSA)
	}

	if firstKey.Use != "sig" {
		return nil, "", "", fmt.Errorf("first key has unsupported use %s, expected 'sig'", firstKey.Use)
	}

	// Extract the Ed25519 private key
	privateKey, ok := firstKey.Key.(ed25519.PrivateKey)
	if !ok {
		return nil, "", "", fmt.Errorf("first key is not an Ed25519 private key")
	}

	return privateKey, privateKeySet.Issuer, firstKey.KeyID, nil
}
