// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var resumeInputProviderCmd = &cobra.Command{
	Use:     "inputprovider",
	Aliases: []string{"rsip", "resourcesetinputprovider"},
	Short:   "Resume ResourceSetInputProvider reconciliation",
	RunE:    resumeInputProviderCmdRun,
}

func init() {
	resumeCmd.AddCommand(resumeInputProviderCmd)
}

func resumeInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := annotateResource(ctx,
		fluxcdv1.ResourceSetInputProviderKind, args[0],
		*kubeconfigArgs.Namespace,
		fluxcdv1.ReconcileAnnotation,
		fluxcdv1.EnabledValue)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Reconciliation resumed")
	return nil
}
