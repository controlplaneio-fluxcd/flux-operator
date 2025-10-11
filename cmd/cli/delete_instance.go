// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var deleteInstanceCmd = &cobra.Command{
	Use:   "instance [name]",
	Short: "Delete FluxInstance from the cluster",
	Example: `  # Uninstall Flux
  flux-operator -n flux-system delete instance flux

  # Delete the instance without uninstalling the Flux components
  flux-operator -n flux-system delete instance flux --with-suspend
`,
	Args:              cobra.ExactArgs(1),
	RunE:              deleteInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

type deleteInstanceFlags struct {
	wait        bool
	withSuspend bool
}

var deleteInstanceArgs deleteInstanceFlags

func init() {
	deleteInstanceCmd.Flags().BoolVar(&deleteInstanceArgs.wait, "wait", true,
		"Wait for the resource to be deleted.")
	deleteInstanceCmd.Flags().BoolVar(&deleteInstanceArgs.withSuspend, "with-suspend", false,
		"Suspend the instance before deletion to prevent uninstalling Flux components.")
	deleteCmd.AddCommand(deleteInstanceCmd)
}

func deleteInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// If --with-suspend is set, suspend the instance before deletion
	// to prevent the operator from uninstalling the Flux components.
	if deleteInstanceArgs.withSuspend {
		now := timeNow()
		if err := toggleSuspension(ctx, gvk, name, *kubeconfigArgs.Namespace, now, true); err != nil {
			return err
		}
		rootCmd.Println(`✔`, "Reconciliation suspended")
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	instance := &fluxcdv1.FluxInstance{}
	instance.SetName(name)
	instance.SetNamespace(*kubeconfigArgs.Namespace)

	if err := kubeClient.Delete(ctx, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%s/%s/%s not found", gvk.Kind, *kubeconfigArgs.Namespace, name)
		}
		return fmt.Errorf("unable to delete %s/%s/%s: %w", gvk.Kind, *kubeconfigArgs.Namespace, name, err)
	}

	if deleteInstanceArgs.wait {
		rootCmd.Println(`◎`, "Waiting for deletion...")
		if err := waitForTermination(ctx, gvk, name, *kubeconfigArgs.Namespace, rootArgs.timeout); err != nil {
			return err
		}
		rootCmd.Println(`✔`, "Deletion completed")
	} else {
		rootCmd.Println(`✔`, "Deletion initiated")
	}

	return nil
}
