// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall [repository]",
	Short: "Uninstall skills from a repository",
	Long:  `The uninstall command removes all skills installed from the specified OCI repository.`,
	Example: `  # Uninstall skills from a repository
  flux-operator skills uninstall ghcr.io/org/agent-skills`,
	Args: cobra.ExactArgs(1),
	RunE: skillsUninstallCmdRun,
}

func init() {
	skillsCmd.AddCommand(skillsUninstallCmd)
}

func skillsUninstallCmdRun(cmd *cobra.Command, args []string) error {
	repo := agentops.NormalizeRepository(args[0])

	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	// Find the matching source.
	srcIdx := -1
	for i, src := range catalog.Spec.Sources {
		if src.Repository == repo {
			srcIdx = i
			break
		}
	}

	if srcIdx < 0 {
		return fmt.Errorf("no skills installed from %s", repo)
	}

	// Remove skill directories.
	if entry, _ := catalog.Status.FindInventoryEntry(repo); entry != nil {
		for _, skill := range entry.Skills {
			if err := agentops.ValidateSkillName(skill.Name); err != nil {
				return fmt.Errorf("invalid skill name in catalog: %w", err)
			}
			skillDir := filepath.Join(skillsDir, skill.Name)
			if err := os.RemoveAll(skillDir); err != nil {
				return fmt.Errorf("removing skill %q: %w", skill.Name, err)
			}
		}
	}

	// Remove source entry.
	catalog.Spec.Sources = append(catalog.Spec.Sources[:srcIdx], catalog.Spec.Sources[srcIdx+1:]...)

	// Remove inventory entry.
	if _, invIdx := catalog.Status.FindInventoryEntry(repo); invIdx >= 0 {
		catalog.Status.Inventory = append(catalog.Status.Inventory[:invIdx], catalog.Status.Inventory[invIdx+1:]...)
	}

	// Save or remove catalog files.
	if len(catalog.Spec.Sources) == 0 {
		for _, name := range []string{agentops.CatalogFileName, agentops.CatalogLockFileName} {
			if err := os.Remove(filepath.Join(skillsDir, name)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing %s: %w", name, err)
			}
		}
	} else {
		if err := agentops.SaveCatalog(skillsDir, catalog); err != nil {
			return fmt.Errorf("saving catalog: %w", err)
		}
		if err := agentops.SaveCatalogLock(skillsDir, catalog); err != nil {
			return fmt.Errorf("saving catalog lock: %w", err)
		}
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Uninstalled skills from %s", repo))
	return nil
}
