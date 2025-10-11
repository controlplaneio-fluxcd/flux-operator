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

var deleteResourceSetCmd = &cobra.Command{
	Use:     "resourceset [name]",
	Aliases: []string{"rset"},
	Short:   "Delete ResourceSet from the cluster",
	Example: `  # Delete a ResourceSet and its managed resources
  flux-operator -n flux-system delete rset my-resourceset

  # Delete a ResourceSet with suspend to prevent resource cleanup
  flux-operator -n flux-system delete rset my-resourceset --with-suspend

  # Delete a ResourceSet without waiting for termination
  flux-operator -n flux-system delete rset my-resourceset --wait=false
`,
	Args:              cobra.ExactArgs(1),
	RunE:              deleteResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

type deleteResourceSetFlags struct {
	wait        bool
	withSuspend bool
}

var deleteResourceSetArgs deleteResourceSetFlags

func init() {
	deleteResourceSetCmd.Flags().BoolVar(&deleteResourceSetArgs.wait, "wait", true,
		"Wait for the resource to be deleted.")
	deleteResourceSetCmd.Flags().BoolVar(&deleteResourceSetArgs.withSuspend, "with-suspend", false,
		"Suspend the resourceset before deletion to prevent resource cleanup.")
	deleteCmd.AddCommand(deleteResourceSetCmd)
}

func deleteResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// If --with-suspend is set, suspend the resourceset before deletion
	// to prevent the operator from cleaning up the managed resources.
	if deleteResourceSetArgs.withSuspend {
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

	resourceSet := &fluxcdv1.ResourceSet{}
	resourceSet.SetName(name)
	resourceSet.SetNamespace(*kubeconfigArgs.Namespace)

	if err := kubeClient.Delete(ctx, resourceSet); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%s/%s/%s not found", gvk.Kind, *kubeconfigArgs.Namespace, name)
		}
		return fmt.Errorf("unable to delete %s/%s/%s: %w", gvk.Kind, *kubeconfigArgs.Namespace, name, err)
	}

	if deleteResourceSetArgs.wait {
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
