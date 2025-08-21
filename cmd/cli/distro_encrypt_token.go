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

var distroEncryptTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Encrypt tokens using JWE with ECDH-ES+A128KW",
	Example: `  # Encrypt a license using the first public key in set
  flux-operator distro encrypt token \
  --key-set=/path/to/enc-public.jwks \
  --input=license-key.jwt \
  --output=license-key.jwe

  # Encrypt from stdin using a specific public key ID
  echo "$GITHUB_TOKEN" | flux-operator distro encrypt token \
  --key-set=/path/to/enc-public.jwks \
  --key-id=12345678-1234-1234-1234-123456789abc \
  --input=/dev/stdin \
  --output=pat.jwe
`,
	RunE: distroEncryptTokenCmdRun,
}

type distroEncryptTokenFlags struct {
	keySetPath string
	keyID      string
	inputPath  string
	outputPath string
}

var distroEncryptTokenArgs distroEncryptTokenFlags

func init() {
	distroEncryptTokenCmd.Flags().StringVarP(&distroEncryptTokenArgs.keySetPath, "key-set", "k", "",
		"path to public key set JWKS file or set the environment variable "+distroEncPublicKeySetEnvVar)
	distroEncryptTokenCmd.Flags().StringVar(&distroEncryptTokenArgs.keyID, "key-id", "",
		"specific key ID to use from the key set (optional, uses first suitable key if not specified)")
	distroEncryptTokenCmd.Flags().StringVarP(&distroEncryptTokenArgs.inputPath, "input", "i", "",
		"path to input file or /dev/stdin (required)")
	distroEncryptTokenCmd.Flags().StringVarP(&distroEncryptTokenArgs.outputPath, "output", "o", "",
		"path to output file or /dev/stdout (required)")

	distroEncryptCmd.AddCommand(distroEncryptTokenCmd)
}

func distroEncryptTokenCmdRun(cmd *cobra.Command, args []string) error {
	// Load public key set
	jwksData, err := loadKeySet(distroEncryptTokenArgs.keySetPath, distroEncPublicKeySetEnvVar)
	if err != nil {
		return err
	}

	var publicKeySet jose.JSONWebKeySet
	err = json.Unmarshal(jwksData, &publicKeySet)
	if err != nil {
		return fmt.Errorf("failed to parse public key set: %w", err)
	}

	// Read input data
	inputData, err := os.ReadFile(distroEncryptTokenArgs.inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	// Encrypt the data
	jweToken, err := lkm.EncryptTokenWithKeySet(inputData, &publicKeySet, distroEncryptTokenArgs.keyID)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Write output
	if distroEncryptTokenArgs.outputPath == "/dev/stdout" {
		fmt.Println(jweToken)
	} else {
		err = os.WriteFile(distroEncryptTokenArgs.outputPath, []byte(jweToken), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		rootCmd.Printf("✔ encrypted data written to: %s\n", distroEncryptTokenArgs.outputPath)
	}

	return nil
}
