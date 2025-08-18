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

	// Generate issuer ID
	issuerID := fmt.Sprintf("%08x", adler32.Checksum([]byte(issuer)))

	privateKeySetPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-private.jwks", issuerID))
	publicKeyKeySetPath := path.Join(distroKeygenArgs.outputDir, fmt.Sprintf("%s-public.jwks", issuerID))

	publicKeyKeySet, privateKeySet, err := lkm.NewKeySetPair(issuer)
	if err != nil {
		return err
	}

	if err := privateKeySet.WriteFile(privateKeySetPath); err != nil {
		return fmt.Errorf("failed to write private key set: %w", err)
	}

	if err := publicKeyKeySet.WriteFile(publicKeyKeySetPath); err != nil {
		return fmt.Errorf("failed to write public key set: %w", err)
	}

	rootCmd.Printf("✔ private key set written to: %s\n", privateKeySetPath)
	rootCmd.Printf("✔ public key set written to: %s\n", publicKeyKeySetPath)

	return nil
}
