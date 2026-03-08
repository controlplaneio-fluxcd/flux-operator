// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

func TestDefaultSkillsDir(t *testing.T) {
	t.Run("returns path relative to cwd", func(t *testing.T) {
		g := NewWithT(t)

		dir, err := agentops.DefaultSkillsDir()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(dir).To(HaveSuffix(agentops.DefaultSkillsDirName))
		g.Expect(filepath.IsAbs(dir)).To(BeTrue())
	})

	t.Run("errors on symlink", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		// Create a real target and a symlink at .agents/skills.
		target := filepath.Join(tmpDir, "target")
		g.Expect(os.MkdirAll(target, 0o755)).To(Succeed())

		agentsDir := filepath.Join(tmpDir, ".agents")
		g.Expect(os.MkdirAll(agentsDir, 0o755)).To(Succeed())
		g.Expect(os.Symlink(target, filepath.Join(agentsDir, "skills"))).To(Succeed())

		// Change to tmpDir so DefaultSkillsDir resolves relative to it.
		origDir, err := os.Getwd()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(os.Chdir(tmpDir)).To(Succeed())
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		_, err = agentops.DefaultSkillsDir()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("symlink"))
	})

	t.Run("errors when path is a file", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()

		// Create .agents/skills as a regular file.
		agentsDir := filepath.Join(tmpDir, ".agents")
		g.Expect(os.MkdirAll(agentsDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(agentsDir, "skills"), []byte("not a dir"), 0o644)).To(Succeed())

		origDir, err := os.Getwd()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(os.Chdir(tmpDir)).To(Succeed())
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		_, err = agentops.DefaultSkillsDir()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("not a directory"))
	})
}

func TestLoadSaveCatalog(t *testing.T) {
	t.Run("returns empty catalog when file not found", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		catalog, err := agentops.LoadCatalog(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(catalog.Kind).To(Equal(fluxcdv1.AgentCatalogKind))
		g.Expect(catalog.APIVersion).To(Equal(fluxcdv1.AgentGroupVersion.String()))
		g.Expect(catalog.Spec.Sources).To(BeEmpty())
	})

	t.Run("round-trips catalog", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		repo := "ghcr.io/test/skills"
		original := &fluxcdv1.AgentCatalog{}
		original.APIVersion = fluxcdv1.AgentGroupVersion.String()
		original.Kind = fluxcdv1.AgentCatalogKind
		original.Spec.Sources = []fluxcdv1.AgentCatalogSource{
			{
				Repository: repo,
				Tag:        "latest",
			},
		}
		original.Status.Inventory = []fluxcdv1.AgentCatalogInventoryEntry{
			{
				ID:           fluxcdv1.RepositoryID(repo),
				URL:          "ghcr.io/test/skills:latest",
				Digest:       "sha256:abc123",
				LastUpdateAt: "2026-01-01T00:00:00Z",
				Skills: []fluxcdv1.AgentCatalogSkill{
					{Name: "my-skill"},
				},
			},
		}

		err := agentops.SaveCatalog(dir, original)
		g.Expect(err).ToNot(HaveOccurred())

		loaded, err := agentops.LoadCatalog(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(loaded.Spec.Sources).To(HaveLen(1))
		g.Expect(loaded.Spec.Sources[0].Repository).To(Equal(repo))
		g.Expect(loaded.Status.Inventory).To(HaveLen(1))

		entry, idx := loaded.Status.FindInventoryEntry(repo)
		g.Expect(idx).To(Equal(0))
		g.Expect(entry).ToNot(BeNil())
		g.Expect(entry.ID).To(Equal(fluxcdv1.RepositoryID(repo)))
		g.Expect(entry.Skills).To(HaveLen(1))
		g.Expect(entry.Skills[0].Name).To(Equal("my-skill"))
	})

	t.Run("errors on invalid YAML", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		// Write invalid YAML as catalog.
		g.Expect(os.WriteFile(filepath.Join(dir, agentops.CatalogFileName), []byte(":::invalid"), 0o644)).To(Succeed())

		_, err := agentops.LoadCatalog(dir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("parsing catalog"))
	})

	t.Run("errors on unreadable file", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		// Create catalog file as a directory to trigger a read error.
		g.Expect(os.MkdirAll(filepath.Join(dir, agentops.CatalogFileName), 0o755)).To(Succeed())

		_, err := agentops.LoadCatalog(dir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("reading catalog"))
	})

	t.Run("errors saving to read-only directory", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()
		g.Expect(os.Chmod(dir, 0o555)).To(Succeed())
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

		catalog := &fluxcdv1.AgentCatalog{}
		catalog.APIVersion = fluxcdv1.AgentGroupVersion.String()
		catalog.Kind = fluxcdv1.AgentCatalogKind

		err := agentops.SaveCatalog(dir, catalog)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("creating temp file"))
	})

	t.Run("atomic write does not corrupt on success", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		catalog := &fluxcdv1.AgentCatalog{}
		catalog.APIVersion = fluxcdv1.AgentGroupVersion.String()
		catalog.Kind = fluxcdv1.AgentCatalogKind

		err := agentops.SaveCatalog(dir, catalog)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the file exists.
		_, err = os.Stat(filepath.Join(dir, agentops.CatalogFileName))
		g.Expect(err).ToNot(HaveOccurred())

		// Verify no temp files remain.
		entries, err := os.ReadDir(dir)
		g.Expect(err).ToNot(HaveOccurred())
		for _, e := range entries {
			g.Expect(e.Name()).ToNot(HavePrefix(".catalog-"))
		}
	})
}

func TestFindInventoryEntry(t *testing.T) {
	t.Run("finds entry by repository", func(t *testing.T) {
		g := NewWithT(t)

		repo := "ghcr.io/test/skills"
		status := fluxcdv1.AgentCatalogStatus{
			Inventory: []fluxcdv1.AgentCatalogInventoryEntry{
				{ID: fluxcdv1.RepositoryID("ghcr.io/other/skills")},
				{ID: fluxcdv1.RepositoryID(repo), Digest: "sha256:abc"},
			},
		}

		entry, idx := status.FindInventoryEntry(repo)
		g.Expect(idx).To(Equal(1))
		g.Expect(entry).ToNot(BeNil())
		g.Expect(entry.Digest).To(Equal("sha256:abc"))
	})

	t.Run("returns nil for missing entry", func(t *testing.T) {
		g := NewWithT(t)

		status := fluxcdv1.AgentCatalogStatus{
			Inventory: []fluxcdv1.AgentCatalogInventoryEntry{
				{ID: fluxcdv1.RepositoryID("ghcr.io/other/skills")},
			},
		}

		entry, idx := status.FindInventoryEntry("ghcr.io/missing/skills")
		g.Expect(idx).To(Equal(-1))
		g.Expect(entry).To(BeNil())
	})
}

func TestCheckSkillConflicts(t *testing.T) {
	t.Run("no conflicts", func(t *testing.T) {
		g := NewWithT(t)

		catalog := &fluxcdv1.AgentCatalog{}
		catalog.Spec.Sources = []fluxcdv1.AgentCatalogSource{
			{Repository: "ghcr.io/other/skills", Tag: "latest"},
		}
		catalog.Status.Inventory = []fluxcdv1.AgentCatalogInventoryEntry{
			{
				ID:     fluxcdv1.RepositoryID("ghcr.io/other/skills"),
				Skills: []fluxcdv1.AgentCatalogSkill{{Name: "other-skill"}},
			},
		}

		err := agentops.CheckSkillConflicts(catalog, "ghcr.io/test/skills", []string{"my-skill"})
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("detects conflicts", func(t *testing.T) {
		g := NewWithT(t)

		catalog := &fluxcdv1.AgentCatalog{}
		catalog.Spec.Sources = []fluxcdv1.AgentCatalogSource{
			{Repository: "ghcr.io/other/skills", Tag: "latest"},
		}
		catalog.Status.Inventory = []fluxcdv1.AgentCatalogInventoryEntry{
			{
				ID:     fluxcdv1.RepositoryID("ghcr.io/other/skills"),
				URL:    "ghcr.io/other/skills:latest",
				Skills: []fluxcdv1.AgentCatalogSkill{{Name: "my-skill"}},
			},
		}

		err := agentops.CheckSkillConflicts(catalog, "ghcr.io/test/skills", []string{"my-skill"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("my-skill"))
		g.Expect(err.Error()).To(ContainSubstring("ghcr.io/other/skills"))
	})

	t.Run("skips same source", func(t *testing.T) {
		g := NewWithT(t)

		catalog := &fluxcdv1.AgentCatalog{}
		catalog.Spec.Sources = []fluxcdv1.AgentCatalogSource{
			{Repository: "ghcr.io/test/skills", Tag: "latest"},
		}
		catalog.Status.Inventory = []fluxcdv1.AgentCatalogInventoryEntry{
			{
				ID:     fluxcdv1.RepositoryID("ghcr.io/test/skills"),
				Skills: []fluxcdv1.AgentCatalogSkill{{Name: "my-skill"}},
			},
		}

		err := agentops.CheckSkillConflicts(catalog, "ghcr.io/test/skills", []string{"my-skill"})
		g.Expect(err).ToNot(HaveOccurred())
	})
}
