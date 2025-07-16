// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var suspendInputProviderCmd = &cobra.Command{
	Use:               "inputprovider [name]",
	Aliases:           []string{"rsip", "resourcesetinputprovider"},
	Short:             "Suspend ResourceSetInputProvider reconciliation",
	Args:              cobra.ExactArgs(1),
	RunE:              suspendInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

func init() {
	suspendCmd.AddCommand(suspendInputProviderCmd)
}

func suspendInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	now := timeNow()
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := toggleSuspension(ctx, gvk, name, *kubeconfigArgs.Namespace, now, true)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Reconciliation suspended")
	return nil
}
