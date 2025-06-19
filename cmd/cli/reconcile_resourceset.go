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

var reconcileResourceSetCmd = &cobra.Command{
	Use:     "resourceset",
	Aliases: []string{"rset"},
	Short:   "Trigger ResourceSet reconciliation",
	Example: `  # Trigger the reconciliation of a ResourceSet
  flux-operator -n flux-system reconcile rset my-resourceset

  # Trigger the reconciliation of a ResourceSet without waiting for it to become ready
  flux-operator -n flux-system reconcile rset my-resourceset --wait=false
`,
	RunE:              reconcileResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

type reconcileResourceSetFlags struct {
	wait bool
}

var reconcileResourceSetArgs reconcileResourceSetFlags

func init() {
	reconcileResourceSetCmd.Flags().BoolVar(&reconcileResourceSetArgs.wait, "wait", true,
		"Wait for the resource to become ready.")
	reconcileCmd.AddCommand(reconcileResourceSetCmd)
}

func reconcileResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	now := metav1.Now().String()
	err := annotateResource(ctx,
		gvk,
		name,
		*kubeconfigArgs.Namespace,
		meta.ReconcileRequestAnnotation,
		now)
	if err != nil {
		return err
	}

	rootCmd.Println(`►`, "Reconciliation triggered")
	if reconcileResourceSetArgs.wait {
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
