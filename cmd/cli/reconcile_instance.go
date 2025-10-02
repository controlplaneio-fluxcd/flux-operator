// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var reconcileInstanceCmd = &cobra.Command{
	Use:   "instance [name]",
	Short: "Trigger FluxInstance reconciliation",
	Example: `  # Trigger the reconciliation of an instance
  flux-operator -n flux-system reconcile instance flux

  # Force the reconciliation by migrating all Flux resources to their latest API version
  flux-operator -n flux-system reconcile instance flux --force

  # Trigger the reconciliation of an instance without waiting for it to become ready
  flux-operator -n flux-system reconcile instance flux --wait=false
`,
	Args:              cobra.ExactArgs(1),
	RunE:              reconcileInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

type reconcileInstanceFlags struct {
	force bool
	wait  bool
}

var reconcileInstanceArgs reconcileInstanceFlags

func init() {
	reconcileInstanceCmd.Flags().BoolVar(&reconcileInstanceArgs.force, "force", false,
		"Force the reconciliation of the instance by migrating all Flux resources to their latest API version")
	reconcileInstanceCmd.Flags().BoolVar(&reconcileInstanceArgs.wait, "wait", true,
		"Wait for the resource to become ready.")

	reconcileCmd.AddCommand(reconcileInstanceCmd)
}

func reconcileInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	now := timeNow()
	annotations := map[string]string{
		meta.ReconcileRequestAnnotation: now,
	}
	if reconcileInstanceArgs.force {
		annotations[meta.ForceRequestAnnotation] = now
	}
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := annotateResourceWithMap(ctx, gvk, name, *kubeconfigArgs.Namespace, annotations)
	if err != nil {
		return err
	}

	rootCmd.Println(`►`, "Reconciliation triggered")
	if reconcileInstanceArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx, gvk, name, *kubeconfigArgs.Namespace, now, rootArgs.timeout)
		if err != nil {
			return err
		}

		rootCmd.Println(`✔`, msg)
	}

	return nil
}
