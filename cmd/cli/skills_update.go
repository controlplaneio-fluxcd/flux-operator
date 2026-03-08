// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/cosign"
)

var skillsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all installed skills",
	Long: `The update command checks for new versions of all installed skills and updates them.
If a skill has been tampered with or deleted locally, it is restored from the upstream artifact.`,
	Example: `  # Update all installed skills
  flux-operator skills update

  # Check for updates without applying them
  flux-operator skills update --dry-run

  # Update with offline signature verification
  flux-operator skills update --verify-trusted-root /path/to/trusted_root.json`,
	Args: cobra.NoArgs,
	RunE: skillsUpdateCmdRun,
}

type skillsUpdateFlags struct {
	verifyTrustedRoot string
	dryRun            bool
}

var skillsUpdateArgs skillsUpdateFlags

func init() {
	skillsUpdateCmd.Flags().StringVar(&skillsUpdateArgs.verifyTrustedRoot, "verify-trusted-root", "",
		"path to a trusted_root.json file for offline signature verification")
	skillsUpdateCmd.Flags().BoolVar(&skillsUpdateArgs.dryRun, "dry-run", false,
		"only check for updates without installing them")

	skillsCmd.AddCommand(skillsUpdateCmd)
}

func skillsUpdateCmdRun(cmd *cobra.Command, args []string) error {
	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	if len(catalog.Spec.Sources) == 0 {
		rootCmd.Println("No skills to update")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	var hadErrors bool
	var updated int

	for _, src := range catalog.Spec.Sources {
		changed, err := updateSource(ctx, catalog, src, skillsDir)
		if err != nil {
			rootCmd.PrintErrf("✗ %v\n", err)
			hadErrors = true
		} else if changed {
			updated++
		}
	}

	if !skillsUpdateArgs.dryRun {
		if err := agentops.SaveCatalog(skillsDir, catalog); err != nil {
			return fmt.Errorf("saving catalog: %w", err)
		}
	}

	if hadErrors {
		return fmt.Errorf("some sources failed to update")
	}

	if updated == 0 {
		rootCmd.Println(`✔`, "All skills are up-to-date")
	}

	if skillsUpdateArgs.dryRun && updated > 0 {
		rootCmd.Println(`✗`, fmt.Sprintf("%d skill source(s) have updates available", updated))
		os.Exit(1)
	}

	return nil
}

// updateSource processes a single source for updates.
// Returns true if the source has pending changes (dry-run) or was updated/restored.
func updateSource(ctx context.Context, catalog *fluxcdv1.AgentCatalog, src fluxcdv1.AgentCatalogSource, skillsDir string) (bool, error) {
	ociURL := fmt.Sprintf("%s:%s", src.Repository, src.Tag)

	// Resolve the remote digest without downloading the artifact.
	rootCmd.Println(`◎`, fmt.Sprintf("Checking %s for updates...", ociURL))
	remoteDigest, err := agentops.ResolveDigest(ctx, ociURL)
	if err != nil {
		return false, err
	}

	// Look up the existing inventory entry once.
	entry, _ := catalog.Status.FindInventoryEntry(src.Repository)

	// Skip if the digest hasn't changed and all skills are intact.
	restoring := false
	if entry != nil && entry.Digest == remoteDigest {
		drifted := agentops.CheckSkillIntegrity(skillsDir, entry.Skills)
		if len(drifted) == 0 {
			rootCmd.Println(`✔`, fmt.Sprintf("%s is up-to-date", src.Repository))
			return false, nil
		}
		rootCmd.Println(`✗`, fmt.Sprintf("%s has drifted skills, restoring...", src.Repository))
		for _, d := range drifted {
			rootCmd.Println(`  •`, d)
		}
		restoring = true
	}

	if skillsUpdateArgs.dryRun {
		if restoring {
			rootCmd.Println(`◎`, fmt.Sprintf("%s has drifted skills that would be restored", src.Repository))
		} else {
			rootCmd.Println(`◎`, fmt.Sprintf("%s has a new version available (%s)", src.Repository, remoteDigest))
		}
		return true, nil
	}

	pinnedURL := fmt.Sprintf("%s@%s", src.Repository, remoteDigest)

	// Verify the new artifact using the digest-pinned reference.
	if err := verifySource(ctx, src, pinnedURL, ociURL); err != nil {
		return false, err
	}

	// Pull the artifact pinned by digest.
	tmpDir, err := os.MkdirTemp("", "skills-update-*")
	if err != nil {
		return false, fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootCmd.Println(`◎`, fmt.Sprintf("Pulling %s...", ociURL))
	if _, err := agentops.PullArtifact(ctx, pinnedURL, tmpDir); err != nil {
		return false, fmt.Errorf("pulling artifact: %w", err)
	}

	// Discover new skills.
	skillNames, err := agentops.DiscoverSkills(tmpDir)
	if err != nil {
		return false, fmt.Errorf("discovering skills: %w", err)
	}
	if len(skillNames) == 0 {
		return false, fmt.Errorf("no skills found in artifact %s", ociURL)
	}

	// Check for conflicts.
	if err := agentops.CheckSkillConflicts(catalog, src.Repository, skillNames); err != nil {
		return false, err
	}

	// Build old checksums map from inventory.
	oldChecksums := make(map[string]string)
	if entry != nil {
		for _, skill := range entry.Skills {
			oldChecksums[skill.Name] = skill.Checksum
		}
	}

	// Diff, sync, and update the catalog inventory.
	changedSkills, skills, err := agentops.SyncSkills(skillsDir, tmpDir, oldChecksums, skillNames, restoring)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	upsertInventoryEntry(catalog, fluxcdv1.AgentCatalogInventoryEntry{
		ID:           fluxcdv1.RepositoryID(src.Repository),
		URL:          ociURL,
		Digest:       remoteDigest,
		LastUpdateAt: now,
		Skills:       skills,
	})

	if restoring {
		rootCmd.Println(`✔`, fmt.Sprintf("Restored %d skill(s) from %s", len(changedSkills), src.Repository))
	} else if len(changedSkills) > 0 {
		rootCmd.Println(`✔`, fmt.Sprintf("Updated %d skill(s) from %s", len(changedSkills), src.Repository))
		for _, name := range changedSkills {
			rootCmd.Println(`  •`, name)
		}
	} else {
		rootCmd.Println(`✔`, fmt.Sprintf("Refreshed %d skill(s) from %s (no content changes)", len(skillNames), src.Repository))
	}
	return true, nil
}

// verifySource verifies the cosign signature of a source artifact if configured.
func verifySource(ctx context.Context, src fluxcdv1.AgentCatalogSource, pinnedURL, ociURL string) error {
	if src.Verify == nil {
		return nil
	}

	oidcIssuer := ""
	oidcSubjectRegex := ""
	if len(src.Verify.MatchOIDCIdentity) > 0 {
		oidcIssuer = src.Verify.MatchOIDCIdentity[0].Issuer
		oidcSubjectRegex = src.Verify.MatchOIDCIdentity[0].Subject
	}

	rootCmd.Println(`◎`, fmt.Sprintf("Verifying %s...", ociURL))
	if err := cosign.VerifyArtifact(ctx, pinnedURL, oidcSubjectRegex, oidcIssuer, skillsUpdateArgs.verifyTrustedRoot); err != nil {
		return fmt.Errorf("signature verification failed for %s: %w", ociURL, err)
	}
	rootCmd.Println(`✔`, "Artifact signature verified")
	return nil
}
