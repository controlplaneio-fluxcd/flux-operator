// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"os"
	"path"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
)

var distroKeygenCmd = &cobra.Command{
	Use:   "keygen [ISSUER]",
	Short: "Generate edDSA key pair in JWK format",
	Example: `  # Generate key pair in the current directory
  flux-operator distro keygen fluxcd.control-plane.io
`,
	Args: cobra.ExactArgs(1),
	RunE: distroKeygenCmdRun,
}

type distroKeygenFlags struct {
	outputDir string
}

var distroKeygenArgs distroKeygenFlags

func init() {
	distroKeygenCmd.Flags().StringVarP(&distroKeygenArgs.outputDir, "output-dir", "o", ".",
		"path to output directory (defaults to current directory)")
	distroCmd.AddCommand(distroKeygenCmd)
}

func distroKeygenCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 || len(args[0]) < 1 {
		return fmt.Errorf("issuer is required")
	}
	issuer := args[0]

	if err := isDir(distroKeygenArgs.outputDir); err != nil {
		return err
	}

	// Generate unique key ID
	checksum := adler32.Checksum([]byte(issuer + timeNow()))
	keyID := fmt.Sprintf("%08x", checksum)

	privateKeySetPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-private.jwks", keyID))
	publicKeyKeySetPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-public.jwks", keyID))

	for _, file := range []string{privateKeySetPath, publicKeyKeySetPath} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			return fmt.Errorf("output file already exists: %s", file)
		}
	}

	// Generate Ed25519 key pair.
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	// Generate PrivateKeySet with the private key
	privateKeySet := PrivateKeySet{
		Issuer: issuer,
		Keys: []jose.JSONWebKey{
			{
				Key:       privateKey,
				KeyID:     keyID,
				Algorithm: string(jose.EdDSA),
				Use:       "sig",
			},
		},
	}

	privateKeySetBytes, err := json.MarshalIndent(privateKeySet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal private key set: %w", err)
	}

	// Generate JWKS (JSON Web Key Set) for the public key
	publicSet := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       publicKey,
				KeyID:     keyID,
				Algorithm: string(jose.EdDSA),
				Use:       "sig",
			},
		},
	}

	publicSetBytes, err := json.MarshalIndent(publicSet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal public key set: %w", err)
	}

	// Write PrivateKeySet and public JWKS to files.
	if err := os.WriteFile(privateKeySetPath, privateKeySetBytes, 0600); err != nil {
		return fmt.Errorf("failed to write private key set to file: %w", err)
	}
	if err := os.WriteFile(publicKeyKeySetPath, publicSetBytes, 0644); err != nil {
		return fmt.Errorf("failed to write public key set to file: %w", err)
	}

	rootCmd.Printf("✔ private key set written to: %s\n", privateKeySetPath)
	rootCmd.Printf("✔ public key set written to: %s\n", publicKeyKeySetPath)

	return nil
}
