// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"
)

var reconcileResourceCmd = &cobra.Command{
	Use:   "resource",
	Short: "Trigger Flux resource reconciliation",
	Example: `  # Trigger the reconciliation of a Flux Kustomization
  flux-operator -n apps reconcile resource Kustomization/my-app

  # Force reconcile a Flux HelmRelease
  flux-operator -n apps reconcile resource HelmRelease/my-release --force

  # Trigger the reconciliation of an OCIRepository without waiting for it to become ready
  flux-operator -n apps reconcile resource OCIRepository/my-app --wait=false
`,
	Args:              cobra.ExactArgs(1),
	RunE:              reconcileResourceCmdRun,
	ValidArgsFunction: resourceKindNameCompletionFunc(true),
}

type reconcileResourceFlags struct {
	force bool
	wait  bool
}

var reconcileeResourceArgs reconcileResourceFlags

func init() {
	reconcileResourceCmd.Flags().BoolVar(&reconcileeResourceArgs.force, "force", false,
		"Force the reconciliation of the resource, applies only to Flux HelmReleases.")
	reconcileResourceCmd.Flags().BoolVar(&reconcileeResourceArgs.wait, "wait", true, "Wait for the resource to become ready.")
	reconcileCmd.AddCommand(reconcileResourceCmd)
}

func reconcileResourceCmdRun(cmd *cobra.Command, args []string) error {
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

	annotations := map[string]string{
		meta.ReconcileRequestAnnotation: now,
	}

	if reconcileeResourceArgs.force {
		annotations[meta.ForceRequestAnnotation] = now
	}

	err = annotateResourceWithMap(ctx, *gvk, name, *kubeconfigArgs.Namespace, annotations)
	if err != nil {
		return err
	}

	rootCmd.Println(`►`, "Reconciliation triggered")
	if reconcileeResourceArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx, *gvk, name, *kubeconfigArgs.Namespace, now, rootArgs.timeout)
		if err != nil {
			return err
		}
		rootCmd.Println(`✔`, msg)
	}
	return nil
}
