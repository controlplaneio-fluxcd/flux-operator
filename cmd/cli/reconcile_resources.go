// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var reconcileResourcesCmd = &cobra.Command{
	Use:     "all",
	Aliases: []string{"resources"},
	Short:   "Trigger Flux resources reconciliation",
	Example: `  # Trigger the reconciliation of all Flux Kustomizations in a namespace
  flux-operator -n apps reconcile all --kind Kustomization

  # Force reconcile all Flux HelmReleases in all namespaces
  flux-operator reconcile all --kind HelmRelease --force -A

  # Trigger the reconciliation of all OCIRepositories in a failed state
  flux-operator reconcile all --kind OCIRepository --ready-status False
`,
	Args: cobra.NoArgs,
	RunE: reconcileResourcesCmdRun,
}

type reconcileResourcesFlags struct {
	kind          string
	force         bool
	readyStatus   string
	allNamespaces bool
}

var reconcileResourcesArgs reconcileResourcesFlags

func init() {
	reconcileResourcesCmd.Flags().StringVar(&reconcileResourcesArgs.kind, "kind", "",
		"The kind of resources to reconcile, e.g., Kustomization, HelmRelease, GitRepository, etc. (required)")
	reconcileResourcesCmd.Flags().BoolVar(&reconcileResourcesArgs.force, "force", false,
		"Force the reconciliation of the resources, applies only to Flux HelmReleases.")
	reconcileResourcesCmd.Flags().StringVar(&reconcileResourcesArgs.readyStatus, "ready-status", "",
		"Filter resources by their ready status, one of: True, False, Unknown.")
	reconcileResourcesCmd.Flags().BoolVarP(&reconcileResourcesArgs.allNamespaces, "all-namespaces", "A", false,
		"Reconcile resources in all namespaces.")
	reconcileCmd.AddCommand(reconcileResourcesCmd)
}

func reconcileResourcesCmdRun(cmd *cobra.Command, args []string) error {
	if reconcileResourcesArgs.kind == "" {
		return fmt.Errorf("--kind is required")
	}
	kind := reconcileResourcesArgs.kind
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

	if reconcileResourcesArgs.force {
		annotations[meta.ForceRequestAnnotation] = now
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	if !reconcileResourcesArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}

	list := unstructured.UnstructuredList{
		Object: map[string]any{
			"apiVersion": gvk.Group + "/" + gvk.Version,
			"kind":       gvk.Kind,
		},
	}

	if err := kubeClient.List(ctx, &list, lsOpts); err != nil {
		return fmt.Errorf("unable to list resources for kind %s : %w", kind, err)
	}

	reconciled := 0
	for i := range list.Items {
		u := &list.Items[i]
		name := u.GetName()
		namespace := u.GetNamespace()
		ready := "Unknown"

		if conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions"); found && err == nil {
			for _, cond := range conditions {
				if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
					ready = condition["status"].(string)
				}
			}
		}

		if reconcileResourcesArgs.readyStatus != "" && !strings.EqualFold(ready, reconcileResourcesArgs.readyStatus) {
			continue
		}

		err = annotateResourceWithMap(ctx, *gvk, name, namespace, annotations)
		if err != nil {
			rootCmd.Printf("✗ Failed to annotate %s/%s: %v\n", namespace, name, err)
		} else {
			rootCmd.Printf("✔ Reconciliation triggered for %s/%s\n", namespace, name)
			reconciled++
		}
	}

	if reconciled == 0 {
		return fmt.Errorf("no resources reconciled of kind %s", kind)
	}

	return nil
}
