// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestSyncAgentSymlinks(t *testing.T) {
	t.Run("creates correct relative symlinks", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		// Create the canonical skills directory with a skill.
		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
		g.Expect(os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o644)).To(Succeed())

		err := SyncAgentSymlinks(root, []string{"claude-code", "kiro"}, []string{"my-skill"})
		g.Expect(err).ToNot(HaveOccurred())

		// Verify claude-code symlink.
		claudeLink := filepath.Join(root, ".claude/skills/my-skill")
		target, err := os.Readlink(claudeLink)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(target).To(Equal(filepath.Join("..", "..", DefaultSkillsDirName, "my-skill")))

		// Verify kiro symlink.
		kiroLink := filepath.Join(root, ".kiro/skills/my-skill")
		target, err = os.Readlink(kiroLink)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(target).To(Equal(filepath.Join("..", "..", DefaultSkillsDirName, "my-skill")))

		// Verify that following the symlink resolves to the real skill.
		info, err := os.Stat(claudeLink)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.IsDir()).To(BeTrue())
	})

	t.Run("skips agents using default skills dir", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())

		err := SyncAgentSymlinks(root, []string{"universal"}, []string{"my-skill"})
		g.Expect(err).ToNot(HaveOccurred())

		// No symlink should exist in the default dir (no extra entry).
		entries, err := os.ReadDir(filepath.Join(root, DefaultSkillsDirName))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entries).To(HaveLen(1)) // only the real skill dir
	})

	t.Run("idempotent on repeated calls", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())

		agents := []string{"claude-code"}
		skills := []string{"my-skill"}

		g.Expect(SyncAgentSymlinks(root, agents, skills)).To(Succeed())
		g.Expect(SyncAgentSymlinks(root, agents, skills)).To(Succeed())

		// Verify the symlink is still correct.
		target, err := os.Readlink(filepath.Join(root, ".claude/skills/my-skill"))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(target).To(Equal(filepath.Join("..", "..", DefaultSkillsDirName, "my-skill")))
	})

	t.Run("errors on non-symlink at target", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())

		// Create a regular directory where the symlink would go.
		conflictDir := filepath.Join(root, ".claude/skills/my-skill")
		g.Expect(os.MkdirAll(conflictDir, 0o755)).To(Succeed())

		err := SyncAgentSymlinks(root, []string{"claude-code"}, []string{"my-skill"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("not a symlink"))
	})
}

func TestRemoveAgentSymlinks(t *testing.T) {
	t.Run("removes symlinks and cleans empty dirs", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())

		agents := []string{"claude-code", "kiro"}
		skills := []string{"my-skill"}

		g.Expect(SyncAgentSymlinks(root, agents, skills)).To(Succeed())

		// Verify symlinks exist.
		_, err := os.Lstat(filepath.Join(root, ".claude/skills/my-skill"))
		g.Expect(err).ToNot(HaveOccurred())

		// Remove them.
		g.Expect(RemoveAgentSymlinks(root, agents, skills)).To(Succeed())

		// Symlinks should be gone.
		_, err = os.Lstat(filepath.Join(root, ".claude/skills/my-skill"))
		g.Expect(os.IsNotExist(err)).To(BeTrue())

		// Empty dirs should be cleaned up.
		_, err = os.Stat(filepath.Join(root, ".claude"))
		g.Expect(os.IsNotExist(err)).To(BeTrue())
		_, err = os.Stat(filepath.Join(root, ".kiro"))
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	})

	t.Run("preserves non-empty agent dirs", func(t *testing.T) {
		g := NewWithT(t)
		root := t.TempDir()

		skillDir := filepath.Join(root, DefaultSkillsDirName, "my-skill")
		g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())

		agents := []string{"claude-code"}
		skills := []string{"my-skill"}

		g.Expect(SyncAgentSymlinks(root, agents, skills)).To(Succeed())

		// Add an extra file in the agent dir.
		g.Expect(os.WriteFile(filepath.Join(root, ".claude/settings.json"), []byte("{}"), 0o644)).To(Succeed())

		g.Expect(RemoveAgentSymlinks(root, agents, skills)).To(Succeed())

		// .claude dir should still exist (has settings.json).
		_, err := os.Stat(filepath.Join(root, ".claude"))
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestAgentSymlinkPrefix(t *testing.T) {
	tests := []struct {
		agentPath string
		expected  string
	}{
		{".claude/skills", filepath.Join("..", "..", ".agents/skills")},
		{".kiro/skills", filepath.Join("..", "..", ".agents/skills")},
		{"skills", filepath.Join("..", ".agents/skills")},
		{".github/skills", filepath.Join("..", "..", ".agents/skills")},
	}

	for _, tt := range tests {
		t.Run(tt.agentPath, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(agentSymlinkPrefix(tt.agentPath)).To(Equal(tt.expected))
		})
	}
}
