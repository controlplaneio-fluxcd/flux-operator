// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/object"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

var treeHelmReleaseCmd = &cobra.Command{
	Use:     "helmrelease [name]",
	Aliases: []string{"hr"},
	Short:   "Print a tree view of the Flux HelmRelease managed objects",
	Example: `  # Print the Kubernetes objects managed by a HelmRelease
  flux-operator -n flux-system tree hr my-app
`,
	Args: cobra.ExactArgs(1),
	RunE: treeHelmReleaseCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(schema.GroupVersionKind{
		Group:   fluxcdv1.FluxHelmGroup,
		Version: "v2",
		Kind:    fluxcdv1.FluxHelmReleaseKind,
	}),
}

func init() {
	treeCmd.AddCommand(treeHelmReleaseCmd)
}

func treeHelmReleaseCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	objMeta := object.ObjMetadata{
		Namespace: *kubeconfigArgs.Namespace,
		Name:      name,
		GroupKind: schema.GroupKind{Group: fluxcdv1.FluxHelmGroup, Kind: fluxcdv1.FluxHelmReleaseKind},
	}

	tree := NewTree(objMeta)
	err = treeFromHelmRelease(ctx, kubeClient, inventory.EntryFromObjMetadata(objMeta, "v2"), tree)
	if err != nil {
		return err
	}

	rootCmd.Println(tree.Print())
	return nil
}

func treeFromHelmRelease(ctx context.Context, kubeClient client.Client, hr fluxcdv1.ResourceRef, tree ObjMetadataTree) error {
	refs, err := inventory.FromHelmRelease(ctx, kubeClient, hr)
	if err != nil {
		return err
	}

	for _, ref := range refs {
		objMetadata, err := inventory.EntryToObjMetadata(ref)
		if err != nil {
			return fmt.Errorf("unable to parse reference from %s: %w", hr.ID, err)
		}

		tree.Add(objMetadata)
	}

	return nil
}
