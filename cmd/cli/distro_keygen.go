// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate asymmetric key pairs for encryption and signing",
}

func init() {
	distroCmd.AddCommand(distroKeygenCmd)
}
