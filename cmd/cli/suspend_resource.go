// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var suspendResourceCmd = &cobra.Command{
	Use:   "resource [kind/name]",
	Short: "suspend Flux resource reconciliation",
	Example: `  # Suspend the reconciliation of a Flux Kustomization
  flux-operator -n apps suspend resource Kustomization/my-app
`,
	Args:              cobra.ExactArgs(1),
	RunE:              suspendResourceCmdRun,
	ValidArgsFunction: resourceKindNameCompletionFunc(true),
}

func init() {
	suspendCmd.AddCommand(suspendResourceCmd)
}

func suspendResourceCmdRun(cmd *cobra.Command, args []string) error {
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

	err = toggleSuspension(ctx, *gvk, name, *kubeconfigArgs.Namespace, now, true)
	if err != nil {
		return err
	}

	rootCmd.Println(`✔`, "Reconciliation suspended")
	return nil
}
