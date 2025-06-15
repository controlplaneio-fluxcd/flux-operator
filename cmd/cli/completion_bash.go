// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generates bash completion scripts",
	Example: `To load completion run

. <(flux-operator completion bash)

To configure your bash shell to load completions for each session add to your bashrc

# ~/.bashrc or ~/.profile
command -v flux-operator >/dev/null && . <(flux-operator completion bash)`,
	Run: func(cmd *cobra.Command, args []string) {
		rootCmd.GenBashCompletion(os.Stdout) //nolint:errcheck
	},
}

func init() {
	completionCmd.AddCommand(completionBashCmd)
}
