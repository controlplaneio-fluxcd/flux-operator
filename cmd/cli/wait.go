// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for Flux Operator resources to become ready",
}

func init() {
	rootCmd.AddCommand(waitCmd)
}
