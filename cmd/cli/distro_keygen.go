// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate JWKs with asymmetric key pairs for signing and encryption",
}

func init() {
	distroCmd.AddCommand(distroKeygenCmd)
}
