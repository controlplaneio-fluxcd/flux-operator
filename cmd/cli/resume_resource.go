// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var resumeResourceCmd = &cobra.Command{
	Use:   "resource [kind/name]",
	Short: "Resume Flux resource reconciliation",
	Example: `  # Resume the reconciliation of a Flux Kustomization
  flux-operator -n apps resume resource Kustomization/my-app
`,
	Args:              cobra.ExactArgs(1),
	RunE:              resumeResourceCmdRun,
	ValidArgsFunction: resourceKindNameCompletionFunc(true),
}

type resumeResourceFlags struct {
	wait bool
}

var resumeResourceArgs resumeResourceFlags

func init() {
	resumeResourceCmd.Flags().BoolVar(&resumeResourceArgs.wait, "wait", true,
		"Wait for the resource to become ready.")
	resumeCmd.AddCommand(resumeResourceCmd)
}

func resumeResourceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("resource name is required")
	}

	parts := strings.Split(args[0], "/")
	if len(parts) != 2 {
		return fmt.Errorf("resource name must be in the format <kind>/<name>, e.g., HelmRelease/my-app")
	}

	kind := parts[0]
	name := parts[1]
	now := timeNow()

	gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("unable to get gvk for kind %s : %w", kind, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err = toggleSuspension(ctx, *gvk, name, *kubeconfigArgs.Namespace, now, false)
	if err != nil {
		return err
	}

	if resumeResourceArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx, *gvk, name, *kubeconfigArgs.Namespace, now, rootArgs.timeout)
		if err != nil {
			return err
		}
		rootCmd.Println(`✔`, msg)
	} else {
		rootCmd.Println(`✔`, "Reconciliation resumed")
	}

	return nil
}
