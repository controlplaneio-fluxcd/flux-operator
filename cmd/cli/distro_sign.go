// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var distroSignCmd = &cobra.Command{
	Use:   "sign",
	Short: "Issue signed license keys and attestations",
}

func init() {
	distroCmd.AddCommand(distroSignCmd)
}
