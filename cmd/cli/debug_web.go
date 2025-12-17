// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var debugWebCmd = &cobra.Command{
	Use:   "web",
	Short: "Debug Flux Operator Web UI features",
}

func init() {
	debugCmd.AddCommand(debugWebCmd)
}
