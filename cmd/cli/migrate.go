// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Kubernetes resources managed fields",
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
