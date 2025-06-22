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
	Use:               "resourceset",
	Aliases:           []string{"rset"},
	Short:             "Resume ResourceSet reconciliation",
	Args:              cobra.ExactArgs(1),
	RunE:              resumeResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

type resumeResourceSetFlags struct {
	wait bool
}

var resumeResourceSetArgs resumeResourceSetFlags

func init() {
	resumeResourceSetCmd.Flags().BoolVar(&resumeResourceSetArgs.wait, "wait", true,
		"Wait for the resource to become ready.")
	resumeCmd.AddCommand(resumeResourceSetCmd)
}

func resumeResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	now := timeNow()
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := toggleSuspension(ctx, gvk, name, *kubeconfigArgs.Namespace, now, false)
	if err != nil {
		return err
	}

	if resumeResourceSetArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx, gvk, name, *kubeconfigArgs.Namespace, now, rootArgs.timeout)
		if err != nil {
			return err
		}

		rootCmd.Println(`✔`, msg)
	} else {
		rootCmd.Println(`✔`, "Reconciliation resumed")
	}

	return nil
}
