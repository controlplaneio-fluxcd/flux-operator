// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash/adler32"
	"os"
	"path"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/spf13/cobra"
)

var distroKeygenCmd = &cobra.Command{
	Use:   "keygen [ISSUER]",
	Short: "Generate an ed25519 key pair for distro signing",
	Example: `  # Generate ed25519 key pair PEM and JWKS in the current directory
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

	// Calculate Adler-32 checksum of issuer to generate key ID
	checksum := adler32.Checksum([]byte(issuer))
	keyID := fmt.Sprintf("%08x", checksum)

	privateKeyPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-private.pem", keyID))
	publicKeyPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-public.pem", keyID))
	publicKeyJSONWebKeyPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-jwks.json", keyID))

	for _, file := range []string{privateKeyPath, publicKeyPath, publicKeyJSONWebKeyPath} {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			return fmt.Errorf("output file already exists: %s", file)
		}
	}

	// Generate Ed25519 key pair.
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ed25519 key: %w", err)
	}

	// Marshal private key to PKCS8 format and encode to PEM with issuer headers.
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	privateKeyPEM := &pem.Block{
		Type: "PRIVATE KEY",
		Headers: map[string]string{
			"KeyID":    keyID,
			"Issuer":   issuer,
			"IssuedAt": time.Now().UTC().Format(time.RFC3339),
		},
		Bytes: privateKeyBytes,
	}
	privatePemData := pem.EncodeToMemory(privateKeyPEM)

	// Marshal public key to PKIX format and encode to PEM.
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicPemData := pem.EncodeToMemory(publicKeyPEM)

	// Generate JWKS (JSON Web Key Set) for the public key
	jwk := jose.JSONWebKey{
		Key:       publicKey,
		KeyID:     keyID,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{jwk},
	}

	jwksBytes, err := json.MarshalIndent(jwks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JWKS: %w", err)
	}

	// Write PEM data and JWKS to files.
	if err := os.WriteFile(privateKeyPath, privatePemData, 0600); err != nil {
		return fmt.Errorf("failed to write private key to file: %w", err)
	}
	if err := os.WriteFile(publicKeyPath, publicPemData, 0644); err != nil {
		return fmt.Errorf("failed to write public key to file: %w", err)
	}
	if err := os.WriteFile(publicKeyJSONWebKeyPath, jwksBytes, 0644); err != nil {
		return fmt.Errorf("failed to write JWKS to file: %w", err)
	}

	rootCmd.Printf("✔ private key written to: %s\n", privateKeyPath)
	rootCmd.Printf("✔ public key written to: %s\n", publicKeyPath)
	rootCmd.Printf("✔ JWKS written to: %s\n", publicKeyJSONWebKeyPath)

	return nil
}
