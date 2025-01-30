// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var resumeResourceSetCmd = &cobra.Command{
	Use:     "resourceset",
	Aliases: []string{"rset"},
	Short:   "Resume ResourceSet reconciliation",
	RunE:    resumeResourceSetCmdRun,
}

func init() {
	resumeCmd.AddCommand(resumeResourceSetCmd)
}

func resumeResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	return annotateResource(ctx,
		fluxcdv1.ResourceSetKind, args[0],
		*kubeconfigArgs.Namespace,
		fluxcdv1.ReconcileAnnotation,
		fluxcdv1.EnabledValue)
}
