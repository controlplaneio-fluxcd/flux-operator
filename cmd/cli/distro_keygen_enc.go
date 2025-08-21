// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"hash/adler32"
	"path"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroKeygenEncCmd = &cobra.Command{
	Use:   "enc [ISSUER]",
	Short: "Generate ECDH-ES+A128KW JWKs for JWE exchange",
	Example: `  # Generate key pair in the current directory
  flux-operator distro keygen enc fluxcd.control-plane.io
`,
	Args: cobra.ExactArgs(1),
	RunE: distroKeygenEncCmdRun,
}

type distroKeygenEncFlags struct {
	outputDir string
}

var distroKeygenEncArgs distroKeygenEncFlags

func init() {
	distroKeygenEncCmd.Flags().StringVarP(&distroKeygenEncArgs.outputDir, "output-dir", "o", ".",
		"path to output directory (defaults to current directory)")
	distroKeygenCmd.AddCommand(distroKeygenEncCmd)
}

func distroKeygenEncCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 || len(args[0]) < 1 {
		return fmt.Errorf("issuer is required")
	}
	issuer := args[0]

	if err := isDir(distroKeygenEncArgs.outputDir); err != nil {
		return err
	}

	// Generate issuer ID
	issuerID := fmt.Sprintf("%08x", adler32.Checksum([]byte(issuer)))

	privateKeySetPath := path.Join(distroKeygenEncArgs.outputDir, fmt.Sprintf("%s-enc-private.jwks", issuerID))
	publicKeySetPath := path.Join(distroKeygenEncArgs.outputDir, fmt.Sprintf("%s-enc-public.jwks", issuerID))

	publicKeySet, privateKeySet, err := lkm.NewEncryptionKeySet()
	if err != nil {
		return err
	}

	if err := lkm.WriteECDHKeySet(privateKeySetPath, privateKeySet); err != nil {
		return fmt.Errorf("failed to write private key set: %w", err)
	}

	if err := lkm.WriteECDHKeySet(publicKeySetPath, publicKeySet); err != nil {
		return fmt.Errorf("failed to write public key set: %w", err)
	}

	rootCmd.Printf("✔ private key set written to: %s\n", privateKeySetPath)
	rootCmd.Printf("✔ public key set written to: %s\n", publicKeySetPath)

	return nil
}
