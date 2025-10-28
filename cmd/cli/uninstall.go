// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/install"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Flux Operator and the Flux instance without affecting reconciled resources",
	Long: `The uninstall command safely removes the Flux Operator and Flux instance from the cluster.

The uninstall command performs the following steps:
  1. Deletes the cluster role bindings of Flux Operator and Flux controllers.
  2. Deletes the deployments of Flux Operator and Flux controllers.
  3. Removes finalizers from Flux Operator and Flux custom resources.
  4. Deletes the CustomResourceDefinitions of Flux Operator and Flux.
  5. Deletes the namespace where Flux Operator is installed (unless --keep-namespace is specified).

The command will not delete any Kubernetes objects or Helm releases that were reconciled on the cluster by Flux.
`,
	Example: `  # Uninstall Flux Operator and Flux instance
  flux-operator -n flux-system uninstall

  # Uninstall but keep the namespace
  flux-operator -n flux-system uninstall --keep-namespace
`,
	Args: cobra.NoArgs,
	RunE: uninstallCmdRun,
}

type uninstallFlags struct {
	keepNamespace bool
}

var uninstallArgs uninstallFlags

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallArgs.keepNamespace, "keep-namespace", false,
		"keep the namespace after uninstalling Flux Operator and Flux instance")

	rootCmd.AddCommand(uninstallCmd)
}

func uninstallCmdRun(cmd *cobra.Command, args []string) error {
	// Set a minimum timeout of 2 minutes
	if rootArgs.timeout < 2*time.Minute {
		rootArgs.timeout = 2 * time.Minute
	}

	now := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("loading kubeconfig failed: %w", err)
	}

	installer, err := install.NewInstaller(ctx, cfg,
		install.WithNamespace(*kubeconfigArgs.Namespace),
		install.WithTerminationTimeout(35*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Step 1: Delete RBAC resources
	// This ensures that the controllers no longer have permissions
	// to reconcile custom resources before we delete them.
	rootCmd.Println(`◎`, "Deleting RBAC resources...")
	rbacResources, err := installer.UninstallRBAC(ctx)
	if err != nil {
		rootCmd.Printf("✗ Failed to delete RBAC resources: %v\n", err)
	} else {
		for _, r := range rbacResources {
			rootCmd.Println(`✔`, r, "deleted")
		}
		rootCmd.Println(`✔`, "RBAC resources deleted successfully")
	}

	// Step 2: Delete controllers
	rootCmd.Println(`◎`, "Deleting controllers...")
	deployments, err := installer.UninstallControllers(ctx)
	if err != nil {
		rootCmd.Printf("✗ Failed to delete controllers: %v\n", err)
	} else {
		for _, d := range deployments {
			rootCmd.Println(`✔`, d, "deleted")
		}
		rootCmd.Println(`✔`, "Controllers deleted successfully")
	}

	// Step 3: Remove finalizers from custom resources
	rootCmd.Println(`◎`, "Removing finalizers from custom resources...")
	if err := installer.RemoveFinalizers(ctx); err != nil {
		rootCmd.Printf("✗ Failed to remove finalizers: %v\n", err)
	} else {
		rootCmd.Println(`✔`, "Finalizers removed successfully")
	}

	// Step 4: Delete CRDs
	rootCmd.Println(`◎`, "Deleting CustomResourceDefinitions...")
	crdNames, err := installer.UninstallCRDs(ctx)
	if err != nil {
		rootCmd.Printf("✗ Failed to delete CRDs: %v\n", err)
	} else {
		for _, c := range crdNames {
			rootCmd.Println(`✔`, c, "deleted")
		}
		rootCmd.Println(`✔`, "CRDs deleted successfully")
	}

	// Step 5: Delete namespace (unless --keep-namespace is specified)
	if !uninstallArgs.keepNamespace {
		rootCmd.Println(`◎`, "Deleting", *kubeconfigArgs.Namespace, "namespace...")
		if err := installer.UninstallNamespace(ctx); err != nil {
			rootCmd.Printf("✗ Failed to delete namespace: %v\n", err)
		} else {
			rootCmd.Println(`✔`, "Namespace deleted successfully")
		}
	}

	rootCmd.Println(`✔`, "Uninstallation completed in", time.Since(now).Round(time.Second).String())
	return nil
}
