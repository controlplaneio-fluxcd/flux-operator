// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroEncryptCmd = &cobra.Command{
	Use:   "encrypt",
	Short: "Encrypt data using JWE with asymmetric key pairs",
}

func init() {
	distroCmd.AddCommand(distroEncryptCmd)
}
