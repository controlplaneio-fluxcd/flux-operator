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

var treeKustomizationCmd = &cobra.Command{
	Use:     "kustomization [name]",
	Aliases: []string{"ks"},
	Short:   "Print a tree view of the Flux Kustomization managed objects",
	Example: `  # Print the Kubernetes objects managed by a Kustomization
  flux-operator -n flux-system tree ks flux-system
`,
	Args: cobra.ExactArgs(1),
	RunE: treeKustomizationCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(schema.GroupVersionKind{
		Group:   fluxcdv1.FluxKustomizeGroup,
		Version: "v1",
		Kind:    fluxcdv1.FluxKustomizationKind,
	}),
}

func init() {
	treeCmd.AddCommand(treeKustomizationCmd)
}

func treeKustomizationCmdRun(cmd *cobra.Command, args []string) error {
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
		GroupKind: schema.GroupKind{Group: fluxcdv1.FluxKustomizeGroup, Kind: fluxcdv1.FluxKustomizationKind},
	}

	tree := NewTree(objMeta)
	err = treeFromKustomization(ctx, kubeClient, inventory.EntryFromObjMetadata(objMeta, "v1"), tree)
	if err != nil {
		return err
	}

	rootCmd.Println(tree.Print())
	return nil
}

func treeFromKustomization(ctx context.Context, kubeClient client.Client, ks fluxcdv1.ResourceRef, tree ObjMetadataTree) error {
	entries, err := inventory.FromStatusOf(ctx, kubeClient, ks)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		objMetadata, err := inventory.EntryToObjMetadata(entry)
		if err != nil {
			return fmt.Errorf("unable to parse reference from %s: %w", ks.ID, err)
		}

		root := tree.Add(objMetadata)

		if fluxcdv1.IsFluxAPI(objMetadata.GroupKind.Group) {
			switch objMetadata.GroupKind.Kind {
			case fluxcdv1.FluxKustomizationKind:
				if entry.ID == ks.ID {
					// Skip self-referencing Kustomization
					continue
				}
				err := treeFromKustomization(ctx, kubeClient, entry, root)
				if err != nil {
					return err
				}
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
			case fluxcdv1.ResourceSetKind:
				err = treeFromResourceSet(ctx, kubeClient, entry, root)
				if err != nil {
					return err
				}
			case fluxcdv1.FluxInstanceKind:
				refs, err := inventory.FromStatusOf(ctx, kubeClient, entry)
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
			}
		}
	}
	return nil
}
