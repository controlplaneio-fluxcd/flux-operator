// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var waitInstanceCmd = &cobra.Command{
	Use:   "instance [name]",
	Short: "Wait for FluxInstance to become ready",
	Example: `  # Wait for an instance to become ready
  flux-operator -n flux-system wait instance flux

  # Wait for an instance to become ready with a custom timeout
  flux-operator -n flux-system wait instance flux --timeout=5m
`,
	Args:              cobra.ExactArgs(1),
	RunE:              waitInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

func init() {
	waitCmd.AddCommand(waitInstanceCmd)
}

func waitInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	rootCmd.Println(`◎`, "Waiting for instance to become ready...")
	msg, err := waitForResourceReconciliation(ctx, gvk, name, *kubeconfigArgs.Namespace, "", rootArgs.timeout)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, msg)
	return nil
}
