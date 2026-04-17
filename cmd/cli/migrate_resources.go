// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/spf13/cobra"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var migrateResourcesCmd = &cobra.Command{
	Use:     "resources",
	Aliases: []string{"resource"},
	Short:   "Migrate the managed fields of Kubernetes resources to a new API version",
	Example: `  # Migrate all ExternalSecret resources to v1 across all namespaces
  flux-operator migrate resources --api-version=external-secrets.io/v1 --kind=ExternalSecret -A

  # List resources that need migration without patching them
  flux-operator migrate resources --api-version=external-secrets.io/v1 --kind=ExternalSecret -A --dry-run

  # Migrate resources in a specific namespace
  flux-operator -n apps migrate resources --api-version=external-secrets.io/v1 --kind=ExternalSecret
`,
	Args: cobra.NoArgs,
	RunE: migrateResourcesCmdRun,
}

type migrateResourcesFlags struct {
	apiVersion    string
	kind          string
	allNamespaces bool
	dryRun        bool
}

var migrateResourcesArgs migrateResourcesFlags

func init() {
	migrateResourcesCmd.Flags().StringVar(&migrateResourcesArgs.apiVersion, "api-version", "",
		"The target API version in the format group/version, e.g., external-secrets.io/v1 (required).")
	migrateResourcesCmd.Flags().StringVar(&migrateResourcesArgs.kind, "kind", "",
		"The kind of resources to migrate, e.g., ExternalSecret (required).")
	migrateResourcesCmd.Flags().BoolVarP(&migrateResourcesArgs.allNamespaces, "all-namespaces", "A", false,
		"Migrate resources in all namespaces.")
	migrateResourcesCmd.Flags().BoolVar(&migrateResourcesArgs.dryRun, "dry-run", false,
		"List the resources that need migration without patching them.")
	migrateCmd.AddCommand(migrateResourcesCmd)
}

func migrateResourcesCmdRun(cmd *cobra.Command, args []string) error {
	if migrateResourcesArgs.apiVersion == "" {
		return fmt.Errorf("--api-version is required")
	}
	if migrateResourcesArgs.kind == "" {
		return fmt.Errorf("--kind is required")
	}

	gv, err := schema.ParseGroupVersion(migrateResourcesArgs.apiVersion)
	if err != nil {
		return fmt.Errorf("invalid --api-version %q: %w", migrateResourcesArgs.apiVersion, err)
	}
	if gv.Group == "" {
		return fmt.Errorf("invalid --api-version %q: group is required", migrateResourcesArgs.apiVersion)
	}
	gvk := gv.WithKind(migrateResourcesArgs.kind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	mapper, err := kubeconfigArgs.ToRESTMapper()
	if err != nil {
		return fmt.Errorf("unable to create REST mapper: %w", err)
	}
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to resolve REST mapping for %s: %w", gvk.String(), err)
	}

	crdName := mapping.Resource.Resource + "." + gvk.Group
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		return fmt.Errorf("unable to get CRD %s: %w", crdName, err)
	}

	if !slices.Contains(crd.Status.StoredVersions, gvk.Version) {
		return fmt.Errorf("version %q is not a stored version of CRD %s (stored versions: %s)",
			gvk.Version, crdName, strings.Join(crd.Status.StoredVersions, ", "))
	}

	list := unstructured.UnstructuredList{
		Object: map[string]any{
			"apiVersion": migrateResourcesArgs.apiVersion,
			"kind":       gvk.Kind + "List",
		},
	}
	lsOpts := &client.ListOptions{}
	if !migrateResourcesArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}
	if err := kubeClient.List(ctx, &list, lsOpts); err != nil {
		return fmt.Errorf("unable to list resources for %s: %w", gvk.String(), err)
	}

	if len(list.Items) == 0 {
		return fmt.Errorf("no resources of kind %s found", gvk.Kind)
	}

	var migrated, failures int

	for i := range list.Items {
		u := &list.Items[i]
		qualified := ssautil.FmtUnstructured(u)

		patches, err := ssa.PatchMigrateToVersion(u, migrateResourcesArgs.apiVersion)
		if err != nil {
			rootCmd.Printf("✗ %s: failed to build migration patch: %v\n", qualified, err)
			failures++
			continue
		}
		if len(patches) == 0 {
			rootCmd.Printf("• %s already at %s\n", qualified, migrateResourcesArgs.apiVersion)
			continue
		}

		if migrateResourcesArgs.dryRun {
			managers := staleManagers(u.GetManagedFields(), migrateResourcesArgs.apiVersion)
			rootCmd.Printf("◎ %s needs migration (managers: %s)\n",
				qualified, strings.Join(managers, ", "))
			migrated++
			continue
		}

		patchBytes, err := json.Marshal(patches)
		if err != nil {
			rootCmd.Printf("✗ %s: failed to marshal patch: %v\n", qualified, err)
			failures++
			continue
		}

		if err := kubeClient.Patch(ctx, u, client.RawPatch(types.JSONPatchType, patchBytes)); err != nil {
			rootCmd.Printf("✗ %s: failed to migrate: %v\n", qualified, err)
			failures++
			continue
		}

		rootCmd.Printf("✔ %s migrated to %s\n", qualified, migrateResourcesArgs.apiVersion)
		migrated++
	}

	if migrateResourcesArgs.dryRun {
		rootCmd.Printf("✔ %d/%d resources need migration to %s\n",
			migrated, len(list.Items), migrateResourcesArgs.apiVersion)
		return nil
	}

	rootCmd.Printf("✔ migrated %d/%d resources to %s\n",
		migrated, len(list.Items), migrateResourcesArgs.apiVersion)

	if failures > 0 && migrated == 0 {
		return fmt.Errorf("failed to migrate any resources (%d errors)", failures)
	}

	return nil
}

func staleManagers(entries []metav1.ManagedFieldsEntry, targetAPIVersion string) []string {
	seen := map[string]struct{}{}
	for _, e := range entries {
		if e.APIVersion != targetAPIVersion {
			seen[e.Manager] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
