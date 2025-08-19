// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke issued signatures",
}

func init() {
	distroCmd.AddCommand(distroRevokeCmd)
}
