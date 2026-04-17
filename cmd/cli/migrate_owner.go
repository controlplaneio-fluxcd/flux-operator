// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

var migrateOwnerCmd = &cobra.Command{
	Use:     "owner [<kind>/<name>]",
	Aliases: []string{"ownership"},
	Short:   "Clean up stale Flux managed fields on resources owned by the given Flux applier",
	Long: `The migrate owner command takes a Flux applier resource (HelmRelease, Kustomization,
ResourceSet, or FluxInstance) and removes stale managed fields entries left behind
by OTHER Flux field managers on every resource listed in its status.inventory.
This is intended to resolve post-migration issues where a field set by a previous
Flux applier is retained even after the resource is reassigned to a new applier.`,
	Example: `  # Clean up stale Flux managers on resources owned by a HelmRelease
  flux-operator -n monitoring migrate owner hr/kube-prom-stack

  # Dry-run: list the stale managers that would be stripped
  flux-operator -n flux-system migrate owner ks/apps --dry-run
`,
	Args: cobra.ExactArgs(1),
	RunE: migrateOwnerCmdRun,
}

type migrateOwnerFlags struct {
	dryRun bool
}

var migrateOwnerArgs migrateOwnerFlags

func init() {
	migrateOwnerCmd.Flags().BoolVar(&migrateOwnerArgs.dryRun, "dry-run", false,
		"List the stale managers that would be stripped without patching resources.")
	migrateCmd.AddCommand(migrateOwnerCmd)
}

var allFluxManagers = []ssa.FieldManager{
	{Name: fluxcdv1.FluxKustomizeController, OperationType: metav1.ManagedFieldsOperationApply, ExactMatch: true},
	{Name: fluxcdv1.FluxHelmController, OperationType: metav1.ManagedFieldsOperationApply, ExactMatch: true},
	// helm-controller (Helm v3 compat) uses Update instead of Apply
	{Name: fluxcdv1.FluxHelmController, OperationType: metav1.ManagedFieldsOperationUpdate, ExactMatch: true},
	{Name: fluxcdv1.FluxOperator, OperationType: metav1.ManagedFieldsOperationApply, ExactMatch: true},
}

func kindToManager(kind string) (string, error) {
	switch kind {
	case fluxcdv1.FluxHelmReleaseKind:
		return fluxcdv1.FluxHelmController, nil
	case fluxcdv1.FluxKustomizationKind:
		return fluxcdv1.FluxKustomizeController, nil
	case fluxcdv1.ResourceSetKind, fluxcdv1.FluxInstanceKind:
		return fluxcdv1.FluxOperator, nil
	default:
		return "", fmt.Errorf("unsupported kind %q, must be one of: %s, %s, %s, %s",
			kind,
			fluxcdv1.FluxHelmReleaseKind,
			fluxcdv1.FluxKustomizationKind,
			fluxcdv1.ResourceSetKind,
			fluxcdv1.FluxInstanceKind)
	}
}

func staleFluxManagers(currentOwner string) []ssa.FieldManager {
	stale := make([]ssa.FieldManager, 0, len(allFluxManagers)-1)
	for _, m := range allFluxManagers {
		if m.Name == currentOwner {
			continue
		}
		stale = append(stale, m)
	}
	return stale
}

func migrateOwnerCmdRun(cmd *cobra.Command, args []string) error {
	owner, err := getObjectByKindName(args)
	if err != nil {
		return err
	}

	currentOwner, err := kindToManager(owner.GetKind())
	if err != nil {
		return err
	}
	staleMgrs := staleFluxManagers(currentOwner)

	entries, err := inventory.FromUnstructured(owner)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("%s/%s/%s has no status.inventory entries (requires Flux v2.8+ for HelmRelease)",
			owner.GetKind(), owner.GetNamespace(), owner.GetName())
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	var cleaned, failures int

	for _, ref := range entries {
		target, err := getInventoryEntry(ctx, kubeClient, ref)
		if err != nil {
			rootCmd.Printf("✗ %s: failed to get resource: %v\n", ref.ID, err)
			failures++
			continue
		}

		qualified := ssautil.FmtUnstructured(target)

		// On HelmRelease targets, preserve the helm-controller Update manager
		// to avoid revoking ownership of status and finalizers.
		targetStaleMgrs := staleMgrs
		if target.GetKind() == fluxcdv1.FluxHelmReleaseKind {
			targetStaleMgrs = make([]ssa.FieldManager, 0, len(staleMgrs))
			for _, m := range staleMgrs {
				if m.Name == fluxcdv1.FluxHelmController && m.OperationType == metav1.ManagedFieldsOperationUpdate {
					continue
				}
				targetStaleMgrs = append(targetStaleMgrs, m)
			}
		}

		patches := ssa.PatchRemoveFieldsManagers(target, targetStaleMgrs)
		if len(patches) == 0 {
			rootCmd.Printf("• %s already clean\n", qualified)
			continue
		}

		removed := removedManagerNames(target.GetManagedFields(), targetStaleMgrs)

		if migrateOwnerArgs.dryRun {
			rootCmd.Printf("◎ %s would strip managers [%s]\n", qualified, strings.Join(removed, ", "))
			cleaned++
			continue
		}

		patchBytes, err := json.Marshal(patches)
		if err != nil {
			rootCmd.Printf("✗ %s: failed to marshal patch: %v\n", qualified, err)
			failures++
			continue
		}

		if err := kubeClient.Patch(ctx, target, client.RawPatch(types.JSONPatchType, patchBytes)); err != nil {
			rootCmd.Printf("✗ %s: failed to strip managers: %v\n", qualified, err)
			failures++
			continue
		}

		rootCmd.Printf("✔ %s stripped managers [%s]\n", qualified, strings.Join(removed, ", "))
		cleaned++
	}

	total := len(entries)
	if migrateOwnerArgs.dryRun {
		rootCmd.Printf("✔ %d/%d resources need cleanup (current owner: %s)\n", cleaned, total, currentOwner)
		return nil
	}

	rootCmd.Printf("✔ cleaned %d/%d resources (current owner: %s)\n", cleaned, total, currentOwner)

	if failures > 0 && cleaned == 0 {
		return fmt.Errorf("failed to process any resource (%d errors)", failures)
	}
	return nil
}

func getInventoryEntry(ctx context.Context, kubeClient client.Client, ref fluxcdv1.ResourceRef) (*unstructured.Unstructured, error) {
	obj, err := inventory.EntryToUnstructured(ref)
	if err != nil {
		return nil, err
	}
	if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func removedManagerNames(entries []metav1.ManagedFieldsEntry, stale []ssa.FieldManager) []string {
	seen := map[string]struct{}{}
	for _, e := range entries {
		for _, s := range stale {
			if e.Manager == s.Name && e.Operation == s.OperationType {
				seen[e.Manager] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
