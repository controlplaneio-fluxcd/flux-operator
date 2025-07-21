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

var treeResourceSetCmd = &cobra.Command{
	Use:     "resourceset [name]",
	Aliases: []string{"rset"},
	Short:   "Print a tree view of the ResourceSet managed objects",
	Example: `  # Print the Kubernetes objects managed by a ResourceSet
  flux-operator -n flux-system tree rset apps
`,
	Args:              cobra.ExactArgs(1),
	RunE:              treeResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

func init() {
	treeCmd.AddCommand(treeResourceSetCmd)
}

func treeResourceSetCmdRun(cmd *cobra.Command, args []string) error {
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
		GroupKind: schema.GroupKind{Group: fluxcdv1.GroupVersion.Group, Kind: fluxcdv1.ResourceSetKind},
	}

	tree := NewTree(objMeta)
	rootEntry := inventory.EntryFromObjMetadata(objMeta, fluxcdv1.GroupVersion.Version)
	err = treeFromResourceSet(ctx, kubeClient, rootEntry, tree)
	if err != nil {
		return err
	}

	rootCmd.Println(tree.Print())
	return nil
}

func treeFromResourceSet(ctx context.Context, kubeClient client.Client, rset fluxcdv1.ResourceRef, tree ObjMetadataTree) error {
	entries, err := inventory.FromStatusOf(ctx, kubeClient, rset)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		objMetadata, err := object.ParseObjMetadata(entry.ID)
		if err != nil {
			return err
		}

		root := tree.Add(objMetadata)

		if fluxcdv1.IsFluxAPI(objMetadata.GroupKind.Group) {
			switch objMetadata.GroupKind.Kind {
			case fluxcdv1.FluxHelmReleaseKind:
				refs, err := inventory.FromHelmRelease(ctx, kubeClient, entry)
				if err != nil {
					return err
				}
				for _, ref := range refs {
					refObjMetadata, err := inventory.EntryToObjMetadata(ref)
					if err != nil {
						return fmt.Errorf("unable to parse reference from %s: %w", objMetadata.String(), err)
					}
					root.Add(refObjMetadata)
				}
			case fluxcdv1.FluxKustomizationKind:
				err = treeFromKustomization(ctx, kubeClient, entry, root)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
