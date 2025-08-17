// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify signatures",
}

func init() {
	distroCmd.AddCommand(distroVerifyCmd)
}
