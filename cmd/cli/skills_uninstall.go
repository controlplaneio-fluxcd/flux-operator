// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall [repository]",
	Short: "Uninstall skills from a repository",
	Long:  `The uninstall command removes all skills installed from the specified OCI repository.`,
	Example: `  # Uninstall skills from a repository
  flux-operator skills uninstall ghcr.io/org/agent-skills

  # Uninstall all skills from all repositories
  flux-operator skills uninstall --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: skillsUninstallCmdRun,
}

type skillsUninstallFlags struct {
	all bool
}

var skillsUninstallArgs skillsUninstallFlags

func init() {
	skillsUninstallCmd.Flags().BoolVar(&skillsUninstallArgs.all, "all", false,
		"uninstall all skills from all repositories")
	skillsCmd.AddCommand(skillsUninstallCmd)
}

func skillsUninstallCmdRun(cmd *cobra.Command, args []string) error {
	if skillsUninstallArgs.all {
		if len(args) > 0 {
			return fmt.Errorf("cannot specify a repository when using --all")
		}
		return skillsUninstallAllRun()
	}

	if len(args) == 0 {
		return fmt.Errorf("requires a repository argument or --all flag")
	}

	return skillsUninstallRepo(args[0])
}

func skillsUninstallAllRun() error {
	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	if len(catalog.Spec.Sources) == 0 {
		return fmt.Errorf("no skills installed")
	}

	// Remove agent symlinks and skill directories for all sources.
	for i, src := range catalog.Spec.Sources {
		entry, _ := catalog.Status.FindInventoryEntry(src.Repository)
		if err := removeSourceSkills(skillsDir, src, entry); err != nil {
			return err
		}
		rootCmd.Println(`✔`, fmt.Sprintf("Uninstalled skills from %s (%d/%d)",
			src.Repository, i+1, len(catalog.Spec.Sources)))
	}

	return removeCatalogFiles(skillsDir)
}

func skillsUninstallRepo(repoArg string) error {
	repo := agentops.NormalizeRepository(repoArg)

	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	// Find the matching source.
	src, srcIdx := catalog.Spec.FindSource(repo)
	if srcIdx < 0 {
		return fmt.Errorf("no skills installed from %s", repo)
	}

	// Look up inventory entry once.
	entry, invIdx := catalog.Status.FindInventoryEntry(repo)

	if err := removeSourceSkills(skillsDir, *src, entry); err != nil {
		return err
	}

	// Remove source entry.
	catalog.Spec.Sources = append(catalog.Spec.Sources[:srcIdx], catalog.Spec.Sources[srcIdx+1:]...)

	// Remove inventory entry.
	if invIdx >= 0 {
		catalog.Status.Inventory = append(catalog.Status.Inventory[:invIdx], catalog.Status.Inventory[invIdx+1:]...)
	}

	// Save or remove catalog files.
	if len(catalog.Spec.Sources) == 0 {
		if err := removeCatalogFiles(skillsDir); err != nil {
			return err
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

// removeSourceSkills removes agent symlinks and skill directories
// for a single catalog source.
func removeSourceSkills(skillsDir string, src fluxcdv1.AgentCatalogSource, entry *fluxcdv1.AgentCatalogInventoryEntry) error {
	if entry != nil && len(src.TargetAgents) > 0 {
		projectRoot, err := agentops.ProjectRoot()
		if err != nil {
			return err
		}
		if err := agentops.RemoveAgentSymlinks(projectRoot, src.TargetAgents, entry.SkillNames()); err != nil {
			return fmt.Errorf("removing agent symlinks for %s: %w", src.Repository, err)
		}
	}

	if entry != nil {
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

	return nil
}

// removeCatalogFiles removes the catalog and catalog lock files,
// then cleans up the skills directory and its parent if empty.
func removeCatalogFiles(skillsDir string) error {
	for _, name := range []string{agentops.CatalogFileName, agentops.CatalogLockFileName} {
		if err := os.Remove(filepath.Join(skillsDir, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", name, err)
		}
	}

	// Clean up empty directories: .agents/skills/, then .agents/.
	agentops.RemoveEmptyDir(skillsDir)
	agentops.RemoveEmptyDir(filepath.Dir(skillsDir))
	return nil
}
