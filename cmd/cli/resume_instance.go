// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var resumeInstanceCmd = &cobra.Command{
	Use:               "instance",
	Short:             "Resume FluxInstance reconciliation",
	RunE:              resumeInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

func init() {
	resumeCmd.AddCommand(resumeInstanceCmd)
}

func resumeInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := annotateResource(ctx,
		gvk,
		name,
		*kubeconfigArgs.Namespace,
		fluxcdv1.ReconcileAnnotation,
		fluxcdv1.EnabledValue)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Reconciliation resumed")
	return nil
}
