// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"os"
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

  # Install a specific version
  flux-operator skills install ghcr.io/org/agent-skills --tag v1.0.0

  # Install from a DockerHub with explicit verification
  flux-operator skills install docker.io/my-org/skills \
    --verify-oidc-issuer=https://token.actions.githubusercontent.com \
    --verify-oidc-subject-regex='^https://github\.com/my-org/skills/.*$'

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

	skillsCmd.AddCommand(skillsInstallCmd)
}

func skillsInstallCmdRun(cmd *cobra.Command, args []string) error {
	repo := agentops.NormalizeRepository(args[0])
	tag := skillsInstallArgs.tag

	oidcIssuer := skillsInstallArgs.verifyOIDCIssuer
	oidcSubjectRegex := skillsInstallArgs.verifyOIDCSubjectRegex

	// Derive OIDC defaults for ghcr.io hosts.
	if skillsInstallArgs.verify && agentops.IsGitHubContainerRegistry(repo) {
		if oidcIssuer == "" {
			oidcIssuer = cosign.DefaultCertOIDCIssuer
		}
		if oidcSubjectRegex == "" {
			owner := agentops.DeriveGitHubOwner(repo)
			oidcSubjectRegex = fmt.Sprintf(`^https://github\.com/%s/.*$`, owner)
		}
	}

	// Require OIDC flags for non-ghcr.io hosts when verify is enabled.
	if skillsInstallArgs.verify && !agentops.IsGitHubContainerRegistry(repo) {
		if oidcIssuer == "" || oidcSubjectRegex == "" {
			return fmt.Errorf("--verify-oidc-issuer and --verify-oidc-subject-regex are required when verification is enabled")
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
	if _, err := agentops.PullArtifact(ctx, pinnedURL, tmpDir); err != nil {
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

	// Build old checksums map from inventory.
	oldChecksums := make(map[string]string)
	if entry, _ := catalog.Status.FindInventoryEntry(repo); entry != nil {
		for _, skill := range entry.Skills {
			oldChecksums[skill.Name] = skill.Checksum
		}
	}

	// Sync skills: remove old, copy new.
	rootCmd.Println(`◎`, "Installing skills...")
	_, skills, err := agentops.SyncSkills(skillsDir, tmpDir, oldChecksums, skillNames, false)
	if err != nil {
		return fmt.Errorf("syncing skills: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	source := fluxcdv1.AgentCatalogSource{
		Repository: repo,
		Tag:        tag,
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
		LastUpdateAt: now,
		Skills:       skills,
	}

	upsertCatalogSource(catalog, source)
	upsertInventoryEntry(catalog, inventoryEntry)

	if err := agentops.SaveCatalog(skillsDir, catalog); err != nil {
		return fmt.Errorf("saving catalog: %w", err)
	}

	rootCmd.Println(`✔`, fmt.Sprintf("Installed %d skill(s) from %s:%s", len(skillNames), repo, tag))
	for _, name := range skillNames {
		rootCmd.Println(`  •`, name)
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
