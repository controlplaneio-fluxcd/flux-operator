// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroDecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt sensitive information using JWE with asymmetric key pairs",
}

func init() {
	distroCmd.AddCommand(distroDecryptCmd)
}
