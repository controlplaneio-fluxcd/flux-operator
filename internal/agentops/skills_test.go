// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "code-review", wantErr: false},
		{name: "valid single char", input: "a", wantErr: false},
		{name: "valid alphanumeric", input: "abc123", wantErr: false},
		{name: "valid with hyphens", input: "my-skill-v2", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "uppercase", input: "Code-Review", wantErr: true},
		{name: "underscores", input: "code_review", wantErr: true},
		{name: "dot dot", input: "..", wantErr: true},
		{name: "slash", input: "a/b", wantErr: true},
		{name: "consecutive hyphens", input: "code--review", wantErr: true},
		{name: "starts with hyphen", input: "-code", wantErr: true},
		{name: "ends with hyphen", input: "code-", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 65), wantErr: true},
		{name: "max length", input: strings.Repeat("a", 64), wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := agentops.ValidateSkillName(tt.input)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestDiscoverSkills(t *testing.T) {
	t.Run("discovers valid skills", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		// Create a valid skill directory.
		skillDir := filepath.Join(dir, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, agentops.SkillFileName), []byte(`---
name: my-skill
description: A test skill
---
# My Skill
`), 0o644)).To(Succeed())

		// Create a directory without SKILL.md (should be skipped).
		g.Expect(os.MkdirAll(filepath.Join(dir, "no-skill"), 0o755)).To(Succeed())

		names, err := agentops.DiscoverSkills(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(names).To(ConsistOf("my-skill"))
	})

	t.Run("errors on name mismatch", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		skillDir := filepath.Join(dir, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, agentops.SkillFileName), []byte(`---
name: wrong-name
description: A test skill
---
`), 0o644)).To(Succeed())

		_, err := agentops.DiscoverSkills(dir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("does not match directory name"))
	})

	t.Run("errors on missing description", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		skillDir := filepath.Join(dir, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, agentops.SkillFileName), []byte(`---
name: my-skill
---
`), 0o644)).To(Succeed())

		_, err := agentops.DiscoverSkills(dir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("missing required 'description' field"))
	})

	t.Run("errors on missing name", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		skillDir := filepath.Join(dir, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, agentops.SkillFileName), []byte(`---
description: A test skill
---
`), 0o644)).To(Succeed())

		_, err := agentops.DiscoverSkills(dir)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("missing required 'name' field"))
	})
}

func TestFilterSkillNames(t *testing.T) {
	t.Run("returns all when targets is empty", func(t *testing.T) {
		g := NewWithT(t)
		discovered := []string{"skill-a", "skill-b", "skill-c"}

		result, err := agentops.FilterSkillNames(discovered, nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(Equal(discovered))
	})

	t.Run("filters to matching names", func(t *testing.T) {
		g := NewWithT(t)
		discovered := []string{"skill-a", "skill-b", "skill-c"}

		result, err := agentops.FilterSkillNames(discovered, []string{"skill-b", "skill-a"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(Equal([]string{"skill-b", "skill-a"}))
	})

	t.Run("errors on missing target", func(t *testing.T) {
		g := NewWithT(t)
		discovered := []string{"skill-a", "skill-b"}

		_, err := agentops.FilterSkillNames(discovered, []string{"skill-a", "nonexistent"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("not found in artifact"))
		g.Expect(err.Error()).To(ContainSubstring("nonexistent"))
	})

	t.Run("errors listing all missing targets", func(t *testing.T) {
		g := NewWithT(t)
		discovered := []string{"skill-a"}

		_, err := agentops.FilterSkillNames(discovered, []string{"missing-1", "missing-2"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("missing-1"))
		g.Expect(err.Error()).To(ContainSubstring("missing-2"))
	})

	t.Run("single target", func(t *testing.T) {
		g := NewWithT(t)
		discovered := []string{"skill-a", "skill-b", "skill-c"}

		result, err := agentops.FilterSkillNames(discovered, []string{"skill-c"})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(result).To(Equal([]string{"skill-c"}))
	})
}

func TestHashSkillDir(t *testing.T) {
	t.Run("produces consistent hash", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		g.Expect(os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644)).To(Succeed())

		hash1, err := agentops.HashSkillDir(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hash1).To(HavePrefix("h1:"))

		hash2, err := agentops.HashSkillDir(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hash2).To(Equal(hash1))
	})

	t.Run("detects changes", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		g.Expect(os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0o644)).To(Succeed())

		hash1, err := agentops.HashSkillDir(dir)
		g.Expect(err).ToNot(HaveOccurred())

		// Modify the file.
		g.Expect(os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0o644)).To(Succeed())

		hash2, err := agentops.HashSkillDir(dir)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(hash2).ToNot(Equal(hash1))
	})
}

func TestCheckSkillIntegrity(t *testing.T) {
	t.Run("all intact", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		// Create two skill directories.
		for _, name := range []string{"skill-a", "skill-b"} {
			skillDir := filepath.Join(dir, name)
			g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
			g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content-"+name), 0o644)).To(Succeed())
		}

		// Compute real checksums.
		checksumA, err := agentops.HashSkillDir(filepath.Join(dir, "skill-a"))
		g.Expect(err).ToNot(HaveOccurred())
		checksumB, err := agentops.HashSkillDir(filepath.Join(dir, "skill-b"))
		g.Expect(err).ToNot(HaveOccurred())

		skills := []fluxcdv1.AgentCatalogSkill{
			{Name: "skill-a", Checksum: checksumA},
			{Name: "skill-b", Checksum: checksumB},
		}

		drifted := agentops.CheckSkillIntegrity(dir, skills)
		g.Expect(drifted).To(BeEmpty())
	})

	t.Run("detects deleted skill", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		// Create only one of two expected skills.
		skillDir := filepath.Join(dir, "skill-a")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("content"), 0o644)).To(Succeed())

		checksumA, err := agentops.HashSkillDir(skillDir)
		g.Expect(err).ToNot(HaveOccurred())

		skills := []fluxcdv1.AgentCatalogSkill{
			{Name: "skill-a", Checksum: checksumA},
			{Name: "skill-b", Checksum: "h1:fake"},
		}

		drifted := agentops.CheckSkillIntegrity(dir, skills)
		g.Expect(drifted).To(HaveLen(1))
		g.Expect(drifted[0].Name).To(Equal("skill-b"))
		g.Expect(drifted[0].Reason).To(Equal("deleted"))
	})

	t.Run("detects modified skill", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		skillDir := filepath.Join(dir, "skill-a")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("original"), 0o644)).To(Succeed())

		originalChecksum, err := agentops.HashSkillDir(skillDir)
		g.Expect(err).ToNot(HaveOccurred())

		// Modify the file after recording the checksum.
		g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("tampered"), 0o644)).To(Succeed())

		skills := []fluxcdv1.AgentCatalogSkill{
			{Name: "skill-a", Checksum: originalChecksum},
		}

		drifted := agentops.CheckSkillIntegrity(dir, skills)
		g.Expect(drifted).To(HaveLen(1))
		g.Expect(drifted[0].Name).To(Equal("skill-a"))
		g.Expect(drifted[0].Reason).To(Equal("modified"))
	})
}

func TestSyncSkills(t *testing.T) {
	t.Run("skips unchanged skills", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := t.TempDir()
		srcDir := t.TempDir()

		// Create identical skill in both source and install dirs.
		for _, dir := range []string{skillsDir, srcDir} {
			skillDir := filepath.Join(dir, "skill-a")
			g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
			g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("same"), 0o644)).To(Succeed())
		}

		checksum, err := agentops.HashSkillDir(filepath.Join(srcDir, "skill-a"))
		g.Expect(err).ToNot(HaveOccurred())

		oldChecksums := map[string]string{"skill-a": checksum}

		changed, skills, err := agentops.SyncSkills(skillsDir, srcDir, oldChecksums, []string{"skill-a"}, false)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(changed).To(BeEmpty())
		g.Expect(skills).To(HaveLen(1))
		g.Expect(skills[0].Name).To(Equal("skill-a"))
		g.Expect(skills[0].Checksum).To(Equal(checksum))
	})

	t.Run("copies changed skills", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := t.TempDir()
		srcDir := t.TempDir()

		// Create skill in source with new content.
		skillDir := filepath.Join(srcDir, "skill-a")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("new-content"), 0o644)).To(Succeed())

		// Old checksum doesn't match new content.
		oldChecksums := map[string]string{"skill-a": "h1:stale"}

		changed, skills, err := agentops.SyncSkills(skillsDir, srcDir, oldChecksums, []string{"skill-a"}, false)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(changed).To(ConsistOf("skill-a"))
		g.Expect(skills).To(HaveLen(1))

		// Verify file was copied.
		data, err := os.ReadFile(filepath.Join(skillsDir, "skill-a", "SKILL.md"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("new-content"))
	})

	t.Run("removes orphaned skills", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := t.TempDir()
		srcDir := t.TempDir()

		// Create orphaned skill in install dir.
		orphanDir := filepath.Join(skillsDir, "old-skill")
		g.Expect(os.MkdirAll(orphanDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(orphanDir, "file.txt"), []byte("orphan"), 0o644)).To(Succeed())

		// Create new skill in source.
		newDir := filepath.Join(srcDir, "new-skill")
		g.Expect(os.MkdirAll(newDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(newDir, "file.txt"), []byte("new"), 0o644)).To(Succeed())

		oldChecksums := map[string]string{"old-skill": "h1:whatever"}

		changed, skills, err := agentops.SyncSkills(skillsDir, srcDir, oldChecksums, []string{"new-skill"}, false)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(changed).To(ConsistOf("new-skill"))
		g.Expect(skills).To(HaveLen(1))

		// Orphan should be removed.
		_, err = os.Stat(orphanDir)
		g.Expect(os.IsNotExist(err)).To(BeTrue())

		// New skill should exist.
		_, err = os.Stat(filepath.Join(skillsDir, "new-skill", "file.txt"))
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("restoring compares against installed dir", func(t *testing.T) {
		g := NewWithT(t)
		skillsDir := t.TempDir()
		srcDir := t.TempDir()

		// Create skill in source.
		srcSkill := filepath.Join(srcDir, "skill-a")
		g.Expect(os.MkdirAll(srcSkill, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(srcSkill, "SKILL.md"), []byte("correct"), 0o644)).To(Succeed())

		// Create tampered version in install dir.
		installedSkill := filepath.Join(skillsDir, "skill-a")
		g.Expect(os.MkdirAll(installedSkill, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(installedSkill, "SKILL.md"), []byte("tampered"), 0o644)).To(Succeed())

		srcChecksum, err := agentops.HashSkillDir(srcSkill)
		g.Expect(err).ToNot(HaveOccurred())

		// Old checksums match the source (same digest), but installed is different.
		oldChecksums := map[string]string{"skill-a": srcChecksum}

		changed, _, err := agentops.SyncSkills(skillsDir, srcDir, oldChecksums, []string{"skill-a"}, true)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(changed).To(ConsistOf("skill-a"))

		// Verify tampered file was replaced.
		data, err := os.ReadFile(filepath.Join(skillsDir, "skill-a", "SKILL.md"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("correct"))
	})
}

func TestCopySkillDir(t *testing.T) {
	t.Run("copies files recursively", func(t *testing.T) {
		g := NewWithT(t)
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dest")

		// Create nested structure.
		g.Expect(os.MkdirAll(filepath.Join(src, "sub"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(src, "sub", "nested.txt"), []byte("nested"), 0o644)).To(Succeed())

		g.Expect(agentops.CopySkillDir(src, dst)).To(Succeed())

		data, err := os.ReadFile(filepath.Join(dst, "root.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("root"))

		data, err = os.ReadFile(filepath.Join(dst, "sub", "nested.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(Equal("nested"))
	})

	t.Run("preserves execute permissions", func(t *testing.T) {
		g := NewWithT(t)
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dest")

		// Create an executable script and a regular file.
		g.Expect(os.WriteFile(filepath.Join(src, "script.sh"), []byte("#!/bin/sh\n"), 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(src, "readme.txt"), []byte("hello"), 0o644)).To(Succeed())

		g.Expect(agentops.CopySkillDir(src, dst)).To(Succeed())

		// Script should retain execute bit.
		info, err := os.Stat(filepath.Join(dst, "script.sh"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()&0o111).ToNot(BeZero(), "execute bits should be preserved")

		// Regular file should not have execute bit.
		info, err = os.Stat(filepath.Join(dst, "readme.txt"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()&0o111).To(BeZero(), "non-executable file should not gain execute bits")
	})

	t.Run("skips symlinks", func(t *testing.T) {
		g := NewWithT(t)
		src := t.TempDir()
		dst := filepath.Join(t.TempDir(), "dest")

		g.Expect(os.WriteFile(filepath.Join(src, "real.txt"), []byte("real"), 0o644)).To(Succeed())
		g.Expect(os.Symlink(filepath.Join(src, "real.txt"), filepath.Join(src, "link.txt"))).To(Succeed())

		g.Expect(agentops.CopySkillDir(src, dst)).To(Succeed())

		// Real file should exist.
		_, err := os.Stat(filepath.Join(dst, "real.txt"))
		g.Expect(err).ToNot(HaveOccurred())

		// Symlink should not be copied.
		_, err = os.Lstat(filepath.Join(dst, "link.txt"))
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	})
}
