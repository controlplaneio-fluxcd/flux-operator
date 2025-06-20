// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var reconcileInputProviderCmd = &cobra.Command{
	Use:     "inputprovider",
	Aliases: []string{"rsip", "resourcesetinputprovider"},
	Short:   "Trigger ResourceSetInputProvider reconciliation",
	Example: `  # Trigger the reconciliation of a ResourceSetInputProvider
  flux-operator -n flux-system reconcile rsip my-inputprovider

  # Force the reconciliation of a ResourceSetInputProvider
  flux-operator -n flux-system reconcile rsip my-inputprovider --force

  # Trigger the reconciliation of a ResourceSetInputProvider without waiting for it to become ready
  flux-operator -n flux-system reconcile rsip my-inputprovider --wait=false
`,
	RunE:              reconcileInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

type reconcileInputProviderFlags struct {
	force bool
	wait  bool
}

var reconcileInputProviderArgs reconcileInputProviderFlags

func init() {
	reconcileInputProviderCmd.Flags().BoolVar(&reconcileInputProviderArgs.force, "force", false,
		"Force the reconciliation of the ResourceSetInputProvider, even if the current time is outside the schedule.")
	reconcileInputProviderCmd.Flags().BoolVar(&reconcileInputProviderArgs.wait, "wait", true,
		"Wait for the resource to become ready.")

	reconcileCmd.AddCommand(reconcileInputProviderCmd)
}

func reconcileInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	now := metav1.Now().String()
	annotations := map[string]string{
		meta.ReconcileRequestAnnotation: now,
	}

	if reconcileInputProviderArgs.force {
		annotations[meta.ForceRequestAnnotation] = now
	}

	err := annotateResourceWithMap(ctx,
		gvk,
		name,
		*kubeconfigArgs.Namespace,
		annotations,
	)
	if err != nil {
		return err
	}

	rootCmd.Println(`►`, "Reconciliation triggered")
	if reconcileInputProviderArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx,
			gvk,
			name,
			*kubeconfigArgs.Namespace,
			now,
			rootArgs.timeout)
		if err != nil {
			return err
		}

		rootCmd.Println(`✔`, msg)
	}

	return nil
}
