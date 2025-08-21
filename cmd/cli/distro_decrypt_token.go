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

var distroDecryptTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Decrypt tokens using JWE with ECDH-ES+A128KW",
	Example: `  # Decrypt a license using the private key set
  flux-operator distro decrypt token \
  --key-set=/path/to/enc-private.jwks \
  --input=license-key.jwe \
  --output=license-key.jwt

  # Decrypt from stdin to stdout
  cat pat.jwe | flux-operator distro decrypt token \
  --key-set=/path/to/enc-private.jwks \
  --input=/dev/stdin \
  --output=/dev/stdout
`,
	RunE: distroDecryptTokenCmdRun,
}

type distroDecryptTokenFlags struct {
	keySetPath string
	inputPath  string
	outputPath string
}

var distroDecryptTokenArgs distroDecryptTokenFlags

func init() {
	distroDecryptTokenCmd.Flags().StringVarP(&distroDecryptTokenArgs.keySetPath, "key-set", "k", "",
		"path to private key set JWKS file or set the environment variable "+distroEncPrivateKeySetEnvVar)
	distroDecryptTokenCmd.Flags().StringVarP(&distroDecryptTokenArgs.inputPath, "input", "i", "",
		"path to input JWE file or /dev/stdin (required)")
	distroDecryptTokenCmd.Flags().StringVarP(&distroDecryptTokenArgs.outputPath, "output", "o", "",
		"path to output file or /dev/stdout (required)")

	distroDecryptCmd.AddCommand(distroDecryptTokenCmd)
}

func distroDecryptTokenCmdRun(cmd *cobra.Command, args []string) error {
	// Load private key set
	jwksData, err := loadKeySet(distroDecryptTokenArgs.keySetPath, distroEncPrivateKeySetEnvVar)
	if err != nil {
		return err
	}

	var privateKeySet jose.JSONWebKeySet
	err = json.Unmarshal(jwksData, &privateKeySet)
	if err != nil {
		return fmt.Errorf("failed to parse private key set: %w", err)
	}

	// Read input JWE data
	jweData, err := os.ReadFile(distroDecryptTokenArgs.inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input JWE: %w", err)
	}

	// Decrypt the data
	decryptedData, err := lkm.DecryptTokenWithKeySet(jweData, &privateKeySet)
	if err != nil {
		return fmt.Errorf("failed to decrypt data: %w", err)
	}

	// Write output
	if distroDecryptTokenArgs.outputPath == "/dev/stdout" {
		fmt.Print(string(decryptedData))
	} else {
		err = os.WriteFile(distroDecryptTokenArgs.outputPath, decryptedData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		rootCmd.Printf("✔ decrypted data written to: %s\n", distroDecryptTokenArgs.outputPath)
	}

	return nil
}
