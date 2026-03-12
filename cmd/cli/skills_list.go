// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

var skillsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed skills",
	Long:    `The list command displays all skills installed in the .agents/skills directory.`,
	Args:    cobra.NoArgs,
	RunE:    skillsListCmdRun,
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
}

func skillsListCmdRun(cmd *cobra.Command, args []string) error {
	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	if len(catalog.Status.Inventory) == 0 {
		rootCmd.Println("No skills installed")
		return nil
	}

	header := []string{"Name", "Repository", "Tag", "Digest", "Last Update"}
	var rows [][]string

	for _, src := range catalog.Spec.Sources {
		entry, _ := catalog.Status.FindInventoryEntry(src.Repository)
		if entry == nil {
			continue
		}

		digest := truncateDigest(entry.Digest)

		for _, skill := range entry.Skills {
			rows = append(rows, []string{
				skill.Name,
				src.Repository,
				src.Tag,
				digest,
				entry.LastUpdateAt,
			})
		}
	}

	if len(rows) == 0 {
		rootCmd.Println("No skills installed")
		return nil
	}

	printTable(rootCmd.OutOrStdout(), header, rows)
	return nil
}

// truncateDigest shortens a digest string to 19 characters for display.
func truncateDigest(digest string) string {
	if len(digest) > 19 {
		return fmt.Sprintf("%s...", digest[:19])
	}
	return digest
}
