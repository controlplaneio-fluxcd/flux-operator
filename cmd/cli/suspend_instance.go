// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var suspendInstanceCmd = &cobra.Command{
	Use:               "instance [name]",
	Short:             "Suspend FluxInstance reconciliation",
	Args:              cobra.ExactArgs(1),
	RunE:              suspendInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

func init() {
	suspendCmd.AddCommand(suspendInstanceCmd)
}

func suspendInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	now := timeNow()
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := toggleSuspension(ctx, gvk, name, *kubeconfigArgs.Namespace, now, true)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Reconciliation suspended")
	return nil
}
