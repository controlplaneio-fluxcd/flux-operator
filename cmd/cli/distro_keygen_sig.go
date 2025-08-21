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

var distroKeygenSigCmd = &cobra.Command{
	Use:   "sig [ISSUER]",
	Short: "Generate EdDSA key pair in JWKS format for signing and verification",
	Example: `  # Generate key pair in the current directory
  flux-operator distro keygen sig https://fluxcd.control-plane.io
`,
	Args: cobra.ExactArgs(1),
	RunE: distroKeygenSigCmdRun,
}

type distroKeygenSigFlags struct {
	outputDir string
}

var distroKeygenSigArgs distroKeygenSigFlags

func init() {
	distroKeygenSigCmd.Flags().StringVarP(&distroKeygenSigArgs.outputDir, "output-dir", "o", ".",
		"path to output directory (defaults to current directory)")
	distroKeygenCmd.AddCommand(distroKeygenSigCmd)
}

func distroKeygenSigCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 || len(args[0]) < 1 {
		return fmt.Errorf("issuer is required")
	}
	issuer := args[0]

	if err := isDir(distroKeygenSigArgs.outputDir); err != nil {
		return err
	}

	// Generate issuer ID
	issuerID := fmt.Sprintf("%08x", adler32.Checksum([]byte(issuer)))

	privateKeySetPath := path.Join(distroKeygenSigArgs.outputDir, fmt.Sprintf("%s-sig-private.jwks", issuerID))
	publicKeyKeySetPath := path.Join(distroKeygenSigArgs.outputDir, fmt.Sprintf("%s-sig-public.jwks", issuerID))

	publicKeyKeySet, privateKeySet, err := lkm.NewSigningKeySet(issuer)
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
