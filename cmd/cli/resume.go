// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume Flux Operator resources reconciliation",
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}
