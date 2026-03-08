// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

// withSkillsDir changes to a temp directory for the duration of the test
// so that skills commands operate on an isolated .agents/skills directory.
// Returns the absolute path to the skills directory.
func withSkillsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	skillsDir := filepath.Join(tmpDir, agentops.DefaultSkillsDirName)
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return skillsDir
}

// seedCatalog creates a catalog with one source and its inventory entry,
// along with the skill directories on disk. Returns the skill names.
func seedCatalog(t *testing.T, skillsDir string) []string {
	t.Helper()
	g := NewWithT(t)

	repo := "ghcr.io/test/agent-skills"
	skillNames := []string{"code-review", "deploy-helper"}

	for _, name := range skillNames {
		skillDir := filepath.Join(skillsDir, name)
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(
			filepath.Join(skillDir, agentops.SkillFileName),
			[]byte("---\nname: "+name+"\ndescription: test\n---\n# "+name+"\n"),
			0o644,
		)).To(Succeed())
	}

	skills := make([]fluxcdv1.AgentCatalogSkill, len(skillNames))
	for i, name := range skillNames {
		checksum, err := agentops.HashSkillDir(filepath.Join(skillsDir, name))
		g.Expect(err).ToNot(HaveOccurred())
		skills[i] = fluxcdv1.AgentCatalogSkill{Name: name, Checksum: checksum}
	}

	catalog := &fluxcdv1.AgentCatalog{}
	catalog.APIVersion = fluxcdv1.AgentGroupVersion.String()
	catalog.Kind = fluxcdv1.AgentCatalogKind
	catalog.Spec.Sources = []fluxcdv1.AgentCatalogSource{
		{Repository: repo, Tag: "latest"},
	}
	catalog.Status.Inventory = []fluxcdv1.AgentCatalogInventoryEntry{
		{
			ID:           fluxcdv1.RepositoryID(repo),
			URL:          repo + ":latest",
			Digest:       "sha256:abc123def456",
			LastUpdateAt: "2026-01-01T00:00:00Z",
			Skills:       skills,
		},
	}

	g.Expect(agentops.SaveCatalog(skillsDir, catalog)).To(Succeed())
	return skillNames
}

func TestSkillsListCmd(t *testing.T) {
	t.Run("no catalog shows no skills", func(t *testing.T) {
		g := NewWithT(t)
		withSkillsDir(t)

		output, err := executeCommand([]string{"skills", "list"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("No skills installed"))
	})

	t.Run("lists installed skills", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := withSkillsDir(t)
		skillNames := seedCatalog(t, skillsDir)

		output, err := executeCommand([]string{"skills", "list"})
		g.Expect(err).ToNot(HaveOccurred())

		for _, name := range skillNames {
			g.Expect(output).To(ContainSubstring(name))
		}
		g.Expect(output).To(ContainSubstring("ghcr.io/test/agent-skills"))
		g.Expect(output).To(ContainSubstring("latest"))
		g.Expect(output).To(ContainSubstring("sha256:abc123def456"))
	})
}

func TestSkillsInstallCmd(t *testing.T) {
	t.Run("requires OIDC flags for non-ghcr hosts", func(t *testing.T) {
		g := NewWithT(t)

		_, err := executeCommand([]string{
			"skills", "install", "docker.io/org/skills",
			"--verify=true",
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("--verify-oidc-issuer"))
	})

	t.Run("requires repository argument", func(t *testing.T) {
		g := NewWithT(t)

		_, err := executeCommand([]string{"skills", "install"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("accepts 1 arg"))
	})
}

func TestSkillsUninstallCmd(t *testing.T) {
	t.Run("errors when no skills installed from repo", func(t *testing.T) {
		g := NewWithT(t)
		withSkillsDir(t)

		_, err := executeCommand([]string{
			"skills", "uninstall", "ghcr.io/nonexistent/skills",
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no skills installed from"))
	})

	t.Run("uninstalls skills and removes catalog", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := withSkillsDir(t)
		skillNames := seedCatalog(t, skillsDir)

		output, err := executeCommand([]string{
			"skills", "uninstall", "ghcr.io/test/agent-skills",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("Uninstalled skills from"))

		// Verify skill directories were removed.
		for _, name := range skillNames {
			_, err := os.Stat(filepath.Join(skillsDir, name))
			g.Expect(os.IsNotExist(err)).To(BeTrue(), "skill dir %q should be removed", name)
		}

		// Verify catalog file was removed (last source).
		_, err = os.Stat(filepath.Join(skillsDir, agentops.CatalogFileName))
		g.Expect(os.IsNotExist(err)).To(BeTrue(), "catalog should be removed when no sources remain")
	})

	t.Run("preserves other sources when uninstalling one", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := withSkillsDir(t)
		seedCatalog(t, skillsDir)

		// Add a second source to the catalog.
		catalog, err := agentops.LoadCatalog(skillsDir)
		g.Expect(err).ToNot(HaveOccurred())

		otherRepo := "ghcr.io/other/skills"
		otherSkillDir := filepath.Join(skillsDir, "other-skill")
		g.Expect(os.MkdirAll(otherSkillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(
			filepath.Join(otherSkillDir, agentops.SkillFileName),
			[]byte("---\nname: other-skill\ndescription: other\n---\n"),
			0o644,
		)).To(Succeed())

		checksum, err := agentops.HashSkillDir(otherSkillDir)
		g.Expect(err).ToNot(HaveOccurred())

		catalog.Spec.Sources = append(catalog.Spec.Sources, fluxcdv1.AgentCatalogSource{
			Repository: otherRepo,
			Tag:        "v1",
		})
		catalog.Status.Inventory = append(catalog.Status.Inventory, fluxcdv1.AgentCatalogInventoryEntry{
			ID:           fluxcdv1.RepositoryID(otherRepo),
			URL:          otherRepo + ":v1",
			Digest:       "sha256:other123",
			LastUpdateAt: "2026-02-01T00:00:00Z",
			Skills:       []fluxcdv1.AgentCatalogSkill{{Name: "other-skill", Checksum: checksum}},
		})
		g.Expect(agentops.SaveCatalog(skillsDir, catalog)).To(Succeed())

		// Uninstall the first source.
		_, err = executeCommand([]string{
			"skills", "uninstall", "ghcr.io/test/agent-skills",
		})
		g.Expect(err).ToNot(HaveOccurred())

		// The other source's skill should still be there.
		_, err = os.Stat(otherSkillDir)
		g.Expect(err).ToNot(HaveOccurred(), "other-skill dir should still exist")

		// Catalog should still exist with one source.
		catalog, err = agentops.LoadCatalog(skillsDir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(catalog.Spec.Sources).To(HaveLen(1))
		g.Expect(catalog.Spec.Sources[0].Repository).To(Equal(otherRepo))
		g.Expect(catalog.Status.Inventory).To(HaveLen(1))
	})

	t.Run("requires repository argument", func(t *testing.T) {
		g := NewWithT(t)

		_, err := executeCommand([]string{"skills", "uninstall"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("accepts 1 arg"))
	})
}

func TestSkillsUpdateCmd(t *testing.T) {
	t.Run("no sources shows no skills to update", func(t *testing.T) {
		g := NewWithT(t)
		withSkillsDir(t)

		output, err := executeCommand([]string{"skills", "update"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(output).To(ContainSubstring("No skills to update"))
	})
}
