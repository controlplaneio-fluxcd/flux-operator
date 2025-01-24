// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"

	"github.com/spf13/cobra"
)

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generates zsh completion scripts",
	Example: `To load completion run

. <(flux-operator completion zsh) && compdef _flux-operator flux-operator

To configure your zsh shell to load completions for each session add to your zshrc

# ~/.zshrc or ~/.profile
command -v flux-operator >/dev/null && . <(timoni completion zsh) && compdef _flux-operator flux-operator

or write a cached file in one of the completion directories in your ${fpath}:

echo "${fpath// /\n}" | grep -i completion
flux-operator completion zsh > _flux-operator

mv _flux-operator ~/.oh-my-zsh/completions  # oh-my-zsh
mv _flux-operator ~/.zprezto/modules/completion/external/src/  # zprezto`,
	Run: func(cmd *cobra.Command, args []string) {
		rootCmd.GenZshCompletion(os.Stdout)
	},
}

func init() {
	completionCmd.AddCommand(completionZshCmd)
}
