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

var deleteInputProviderCmd = &cobra.Command{
	Use:     "inputprovider [name]",
	Aliases: []string{"rsip", "resourcesetinputprovider"},
	Short:   "Delete ResourceSetInputProvider from the cluster",
	Example: `  # Delete a ResourceSetInputProvider
  flux-operator -n flux-system delete inputprovider my-inputprovider

  # Delete a ResourceSetInputProvider without waiting
  flux-operator -n flux-system delete rsip my-inputprovider --wait=false
`,
	Args:              cobra.ExactArgs(1),
	RunE:              deleteInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

type deleteInputProviderFlags struct {
	wait bool
}

var deleteInputProviderArgs deleteInputProviderFlags

func init() {
	deleteInputProviderCmd.Flags().BoolVar(&deleteInputProviderArgs.wait, "wait", true,
		"Wait for the resource to be deleted.")
	deleteCmd.AddCommand(deleteInputProviderCmd)
}

func deleteInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	inputProvider := &fluxcdv1.ResourceSetInputProvider{}
	inputProvider.SetName(name)
	inputProvider.SetNamespace(*kubeconfigArgs.Namespace)

	if err := kubeClient.Delete(ctx, inputProvider); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("%s/%s/%s not found", gvk.Kind, *kubeconfigArgs.Namespace, name)
		}
		return fmt.Errorf("unable to delete %s/%s/%s: %w", gvk.Kind, *kubeconfigArgs.Namespace, name, err)
	}

	if deleteInputProviderArgs.wait {
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
