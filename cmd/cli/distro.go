// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var distroCmd = &cobra.Command{
	Use:   "distro",
	Short: "Provides utilities for managing the Flux distribution",
}

func init() {
	rootCmd.AddCommand(distroCmd)
}

const (
	distroPrivateKeySetEnvVar = "FLUX_DISTRO_PRIVATE_KEY_SET"
	distroPublicKeySetEnvVar  = "FLUX_DISTRO_PUBLIC_KEY_SET"
	distroDefaultAudience     = "flux-operator"
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

// loadKeySet reads the JWKS from file path or environment variable
func loadKeySet(keySetPath, envVarName string) ([]byte, error) {
	if keySetPath != "" {
		// Load from file or /dev/stdin
		jwksData, err := os.ReadFile(keySetPath)
		if err != nil {
			return nil, err
		}
		return jwksData, nil
	} else if keyData := os.Getenv(envVarName); keyData != "" {
		// Load from environment variable
		return []byte(keyData), nil
	} else {
		return nil, fmt.Errorf("JWKS must be specified with --key-set flag or %s environment variable",
			envVarName)
	}
}
