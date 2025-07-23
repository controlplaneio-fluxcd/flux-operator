// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var waitInputProviderCmd = &cobra.Command{
	Use:     "inputprovider [name]",
	Aliases: []string{"rsip", "resourcesetinputprovider"},
	Short:   "Wait for ResourceSetInputProvider to become ready",
	Example: `  # Wait for an ResourceSetInputProvider to become ready
  flux-operator -n flux-system wait inputprovider my-inputprovider

  # Wait for an ResourceSetInputProvider to become ready with a custom timeout
  flux-operator -n flux-system wait rsip my-inputprovider --timeout=5m
`,
	Args:              cobra.ExactArgs(1),
	RunE:              waitInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

func init() {
	waitCmd.AddCommand(waitInputProviderCmd)
}

func waitInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	rootCmd.Println(`◎`, "Waiting for inputprovider to become ready...")
	msg, err := waitForResourceReconciliation(ctx, gvk, name, *kubeconfigArgs.Namespace, "", rootArgs.timeout)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, msg)
	return nil
}
