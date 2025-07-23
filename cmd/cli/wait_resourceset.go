// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var waitResourceSetCmd = &cobra.Command{
	Use:     "resourceset [name]",
	Aliases: []string{"rset"},
	Short:   "Wait for ResourceSet to become ready",
	Example: `  # Wait for a resourceset to become ready
  flux-operator -n flux-system wait resourceset my-resourceset

  # Wait for a resourceset to become ready with a custom timeout
  flux-operator -n flux-system wait rset my-resourceset --timeout=5m
`,
	Args:              cobra.ExactArgs(1),
	RunE:              waitResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

func init() {
	waitCmd.AddCommand(waitResourceSetCmd)
}

func waitResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	rootCmd.Println(`◎`, "Waiting for resourceset to become ready...")
	msg, err := waitForResourceReconciliation(ctx, gvk, name, *kubeconfigArgs.Namespace, "", rootArgs.timeout)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, msg)
	return nil
}
