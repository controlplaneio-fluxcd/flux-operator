// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

var distroCmd = &cobra.Command{
	Use:   "distro",
	Short: "Provides utilities for managing the Flux distribution and licensing",
}

func init() {
	rootCmd.AddCommand(distroCmd)
}

const (
	distroSigPrivateKeySetEnvVar = "FLUX_DISTRO_SIG_PRIVATE_JWKS"
	distroSigPublicKeySetEnvVar  = "FLUX_DISTRO_SIG_PUBLIC_JWKS"
	distroEncPrivateKeySetEnvVar = "FLUX_DISTRO_ENC_PRIVATE_JWKS"
	distroEncPublicKeySetEnvVar  = "FLUX_DISTRO_ENC_PUBLIC_JWKS"
	distroDefaultAudience        = "flux-operator"
)

// isDir validates that the given path exists and is a directory
func isDir(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory %s does not exist", path)
	}
	if err != nil {
		return fmt.Errorf("failed to check path %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", path)
	}
	return nil
}

// loadKeySet reads the JWKS from file path, HTTP URL, or environment variable
func loadKeySet(ctx context.Context, keySetPath, envVarName string) ([]byte, error) {
	if keySetPath != "" {
		// Load from HTTP URL
		if strings.HasPrefix(keySetPath, "http://") || strings.HasPrefix(keySetPath, "https://") {
			return lkm.Fetch(ctx, keySetPath, lkm.FetchOpt.WithContentType("application/jwks"))
		}
		// Load from file or /dev/stdin
		return os.ReadFile(keySetPath)
	} else if keyData := os.Getenv(envVarName); keyData != "" {
		// Load from environment variable
		return []byte(keyData), nil
	} else {
		return nil, fmt.Errorf("JWKS must be specified with --key-set flag or %s environment variable",
			envVarName)
	}
}

// loadAttestation reads the attestation JWT from file path or HTTP URL.
func loadAttestation(ctx context.Context, attestationPath string) ([]byte, error) {
	if attestationPath == "" {
		return nil, fmt.Errorf("--attestation flag is required")
	}

	if strings.HasPrefix(attestationPath, "http://") || strings.HasPrefix(attestationPath, "https://") {
		return lkm.Fetch(ctx, attestationPath, lkm.FetchOpt.WithContentType("application/jwt"))
	}
	return os.ReadFile(attestationPath)
}
