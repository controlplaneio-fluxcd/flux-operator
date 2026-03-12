// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/cosign"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var skillsInstallCmd = &cobra.Command{
	Use:   "install [repository]",
	Short: "Install skills from an OCI artifact",
	Long: `The install command downloads skills from an OCI artifact and installs them
to the .agents/skills directory in the current working directory.`,
	Example: `  # Install skills from a ghcr.io repository
  # with default tag 'latest' and signature verification
  flux-operator skills install ghcr.io/org/agent-skills

  # Install only specific skills from an artifact
  flux-operator skills install ghcr.io/org/agent-skills \
    --skill code-review \
    --skill deploy-helper

  # Install a specific version and symlink for specific agents
  flux-operator skills install ghcr.io/org/agent-skills \
  --tag v1.0.0 \
  --agent claude-code \
  --agent kiro

  # Install from a DockerHub with custom verification
  flux-operator skills install docker.io/my-org/skills \
    --verify-oidc-issuer=https://github.com/login/oauth \
    --verify-oidc-subject-regex='^username@example.com$'

  # Install without signature verification
  flux-operator skills install ghcr.io/org/agent-skills --verify=false`,
	Args: cobra.ExactArgs(1),
	RunE: skillsInstallCmdRun,
}

type skillsInstallFlags struct {
	tag                    string
	verify                 bool
	verifyOIDCIssuer       string
	verifyOIDCSubjectRegex string
	verifyTrustedRoot      string
	agents                 []string
	skills                 []string
}

var skillsInstallArgs skillsInstallFlags

func init() {
	skillsInstallCmd.Flags().StringVar(&skillsInstallArgs.tag, "tag", "latest",
		"OCI artifact tag")
	skillsInstallCmd.Flags().BoolVar(&skillsInstallArgs.verify, "verify", true,
		"verify the cosign signature of the artifact")
	skillsInstallCmd.Flags().StringVar(&skillsInstallArgs.verifyOIDCIssuer, "verify-oidc-issuer", "",
		"OIDC issuer for signature verification")
	skillsInstallCmd.Flags().StringVar(&skillsInstallArgs.verifyOIDCSubjectRegex, "verify-oidc-subject-regex", "",
		"OIDC subject regexp for signature verification")
	skillsInstallCmd.Flags().StringVar(&skillsInstallArgs.verifyTrustedRoot, "verify-trusted-root", "",
		"path to a trusted_root.json file for offline signature verification")
	skillsInstallCmd.Flags().StringSliceVar(&skillsInstallArgs.agents, "agent", []string{"universal"},
		"agent ID(s) for which to create skill symlinks (can be specified multiple times)")
	_ = skillsInstallCmd.RegisterFlagCompletionFunc("agent", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return agentops.AgentIDs(), cobra.ShellCompDirectiveNoFileComp
	})
	skillsInstallCmd.Flags().StringSliceVar(&skillsInstallArgs.skills, "skill", nil,
		"skill name(s) to install from the artifact (can be specified multiple times, default: all)")

	skillsCmd.AddCommand(skillsInstallCmd)
}

func skillsInstallCmdRun(cmd *cobra.Command, args []string) error {
	repo := agentops.NormalizeRepository(args[0])
	tag := skillsInstallArgs.tag

	if err := validateAgentIDs(skillsInstallArgs.agents); err != nil {
		return err
	}

	for _, s := range skillsInstallArgs.skills {
		if err := agentops.ValidateSkillName(s); err != nil {
			return fmt.Errorf("invalid --skill value: %w", err)
		}
	}

	var oidcIssuer, oidcSubjectRegex string
	if skillsInstallArgs.verify {
		var err error
		oidcIssuer, oidcSubjectRegex, err = resolveInstallOIDC(repo)
		if err != nil {
			return err
		}
	}

	ociURL := fmt.Sprintf("%s:%s", repo, tag)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	// Resolve the remote digest.
	rootCmd.Println(`◎`, "Resolving artifact digest...")
	digest, err := agentops.ResolveDigest(ctx, ociURL)
	if err != nil {
		return err
	}
	rootCmd.Println(`✔`, fmt.Sprintf("Using digest %s", digest))

	pinnedURL := fmt.Sprintf("%s@%s", repo, digest)

	// Verify the artifact signature using the digest-pinned reference.
	if skillsInstallArgs.verify {
		rootCmd.Println(`◎`, "Verifying artifact signature...")
		if err := cosign.VerifyArtifact(ctx, pinnedURL, oidcSubjectRegex, oidcIssuer, skillsInstallArgs.verifyTrustedRoot); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
		rootCmd.Println(`✔`, "Artifact signature verified")
	}

	// Pull the artifact pinned by digest.
	tmpDir, err := os.MkdirTemp("", "skills-install-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rootCmd.Println(`◎`, "Pulling skills artifact...")
	artifactInfo, err := agentops.PullArtifact(ctx, pinnedURL, tmpDir)
	if err != nil {
		return fmt.Errorf("pulling artifact: %w", err)
	}

	// Discover skills in the extracted artifact.
	skillNames, err := agentops.DiscoverSkills(tmpDir)
	if err != nil {
		return fmt.Errorf("discovering skills: %w", err)
	}
	if len(skillNames) == 0 {
		return fmt.Errorf("no skills found in artifact %s", ociURL)
	}

	// Filter to selected skills if --skill was specified.
	skillNames, err = agentops.FilterSkillNames(skillNames, skillsInstallArgs.skills)
	if err != nil {
		return err
	}

	// Load or create the catalog.
	skillsDir, err := agentops.DefaultSkillsDir()
	if err != nil {
		return err
	}

	catalog, err := agentops.LoadCatalog(skillsDir)
	if err != nil {
		return err
	}

	// Check for skill name conflicts with other sources.
	if err := agentops.CheckSkillConflicts(catalog, repo, skillNames); err != nil {
		return err
	}

	// Create the skills directory if needed.
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	// Collect old state from catalog for re-install handling.
	var oldAgents []string
	if oldSrc, _ := catalog.Spec.FindSource(repo); oldSrc != nil {
		oldAgents = oldSrc.TargetAgents
	}

	oldChecksums := make(map[string]string)
	var oldSkillNames []string
	if entry, _ := catalog.Status.FindInventoryEntry(repo); entry != nil {
		for _, skill := range entry.Skills {
			oldChecksums[skill.Name] = skill.Checksum
		}
		oldSkillNames = entry.SkillNames()
	}

	// Sync skills: remove old, copy new.
	rootCmd.Println(`◎`, "Installing skills...")
	_, skills, err := agentops.SyncSkills(skillsDir, tmpDir, oldChecksums, skillNames, false)
	if err != nil {
		return fmt.Errorf("syncing skills: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	source := fluxcdv1.AgentCatalogSource{
		Repository:   repo,
		Tag:          tag,
		TargetAgents: skillsInstallArgs.agents,
		TargetSkills: skillsInstallArgs.skills,
	}
	if skillsInstallArgs.verify {
		source.Verify = &fluxcdv1.AgentCatalogVerify{
			Provider: fluxcdv1.AgentCatalogVerifyProviderCosign,
		}
		if oidcIssuer != "" || oidcSubjectRegex != "" {
			source.Verify.MatchOIDCIdentity = []fluxcdv1.OIDCIdentity{
				{Issuer: oidcIssuer, Subject: oidcSubjectRegex},
			}
		}
	}

	inventoryEntry := fluxcdv1.AgentCatalogInventoryEntry{
		ID:           fluxcdv1.RepositoryID(repo),
		URL:          fmt.Sprintf("%s:%s", repo, tag),
		Digest:       digest,
		Annotations:  artifactInfo.Annotations,
		LastUpdateAt: now,
		Skills:       skills,
	}

	upsertCatalogSource(catalog, source)
	upsertInventoryEntry(catalog, inventoryEntry)

	if err := agentops.SaveCatalog(skillsDir, catalog); err != nil {
		return fmt.Errorf("saving catalog: %w", err)
	}
	if err := agentops.SaveCatalogLock(skillsDir, catalog); err != nil {
		return fmt.Errorf("saving catalog lock: %w", err)
	}

	projectRoot, err := agentops.ProjectRoot()
	if err != nil {
		return err
	}

	// Remove stale agent symlinks when agents change on re-install.
	if len(oldAgents) > 0 && len(oldSkillNames) > 0 {
		if err := agentops.RemoveAgentSymlinks(projectRoot, oldAgents, oldSkillNames); err != nil {
			return fmt.Errorf("removing old agent symlinks: %w", err)
		}
	}

	// Create symlinks for the new agents.
	if err := agentops.SyncAgentSymlinks(projectRoot, skillsInstallArgs.agents, skillNames); err != nil {
		return fmt.Errorf("creating agent symlinks: %w", err)
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Installed %d skill(s) from %s:%s", len(skillNames), repo, tag))
	for _, name := range skillNames {
		rootCmd.Println(`  •`, name)
	}

	return nil
}

// validateAgentIDs returns an error if any of the given IDs are unknown.
func validateAgentIDs(ids []string) error {
	var unknown []string
	for _, id := range ids {
		if agentops.FindAgent(id) == nil {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown agent ID(s): %s; run with --help to see valid IDs", strings.Join(unknown, ", "))
	}
	return nil
}

// upsertCatalogSource updates or inserts a source in the catalog.
func upsertCatalogSource(catalog *fluxcdv1.AgentCatalog, source fluxcdv1.AgentCatalogSource) {
	for i, src := range catalog.Spec.Sources {
		if src.Repository == source.Repository {
			catalog.Spec.Sources[i] = source
			return
		}
	}
	catalog.Spec.Sources = append(catalog.Spec.Sources, source)
}

// upsertInventoryEntry updates or inserts an inventory entry in the catalog by ID.
func upsertInventoryEntry(catalog *fluxcdv1.AgentCatalog, entry fluxcdv1.AgentCatalogInventoryEntry) {
	for i := range catalog.Status.Inventory {
		if catalog.Status.Inventory[i].ID == entry.ID {
			catalog.Status.Inventory[i] = entry
			return
		}
	}
	catalog.Status.Inventory = append(catalog.Status.Inventory, entry)
}

// resolveInstallOIDC derives or validates OIDC parameters for signature verification.
func resolveInstallOIDC(repo string) (oidcIssuer, oidcSubjectRegex string, err error) {
	oidcIssuer = skillsInstallArgs.verifyOIDCIssuer
	oidcSubjectRegex = skillsInstallArgs.verifyOIDCSubjectRegex

	if agentops.IsGitHubContainerRegistry(repo) {
		if oidcIssuer == "" {
			oidcIssuer = cosign.DefaultCertOIDCIssuer
		}
		if oidcSubjectRegex == "" {
			owner := agentops.DeriveGitHubOwner(repo)
			oidcSubjectRegex = fmt.Sprintf(`^https://github\.com/%s/.*$`, owner)
		}
	} else if oidcIssuer == "" || oidcSubjectRegex == "" {
		return "", "", fmt.Errorf("--verify-oidc-issuer and --verify-oidc-subject-regex are required when verification is enabled")
	}
	return oidcIssuer, oidcSubjectRegex, nil
}
