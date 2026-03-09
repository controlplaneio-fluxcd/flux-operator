// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

// extractTarEntries decompresses tar+gzip data and returns the list of entry names.
func extractTarEntries(t *testing.T, data []byte) []string {
	t.Helper()
	g := NewWithT(t)

	gr, err := gzip.NewReader(bytes.NewReader(data))
	g.Expect(err).ToNot(HaveOccurred())
	defer gr.Close()

	tr := tar.NewReader(gr)
	var entries []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		g.Expect(err).ToNot(HaveOccurred())
		entries = append(entries, header.Name)
	}
	return entries
}

func TestBuildArtifact(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	g.Expect(os.MkdirAll(filepath.Join(dir, "skill-a"), 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "skill-a", "SKILL.md"), []byte("# skill-a"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "skill-a", "instructions.md"), []byte("do stuff"), 0o644)).To(Succeed())

	data, err := BuildArtifact(dir, []string{"skill-a"})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).ToNot(BeEmpty())

	// Verify normalized headers on all entries.
	gr, err := gzip.NewReader(bytes.NewReader(data))
	g.Expect(err).ToNot(HaveOccurred())
	defer gr.Close()

	tr := tar.NewReader(gr)
	var entries []string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		g.Expect(err).ToNot(HaveOccurred())
		entries = append(entries, header.Name)
		g.Expect(header.Uid).To(Equal(0))
		g.Expect(header.Gid).To(Equal(0))
		g.Expect(header.Uname).To(BeEmpty())
		g.Expect(header.Gname).To(BeEmpty())
		g.Expect(header.ModTime.Unix()).To(BeNumerically("<=", 0))
	}

	g.Expect(entries).To(ContainElement("skill-a/"))
	g.Expect(entries).To(ContainElement("skill-a/SKILL.md"))
	g.Expect(entries).To(ContainElement("skill-a/instructions.md"))
}

func TestBuildArtifactSkipsSymlinks(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	g.Expect(os.MkdirAll(skillDir, 0o755)).To(Succeed())
	realFile := filepath.Join(skillDir, "real.txt")
	g.Expect(os.WriteFile(realFile, []byte("real"), 0o644)).To(Succeed())
	g.Expect(os.Symlink(realFile, filepath.Join(skillDir, "link.txt"))).To(Succeed())

	data, err := BuildArtifact(dir, []string{"my-skill"})
	g.Expect(err).ToNot(HaveOccurred())

	entries := extractTarEntries(t, data)
	g.Expect(entries).To(ContainElement("my-skill/real.txt"))
	g.Expect(entries).ToNot(ContainElement("my-skill/link.txt"))
}

func TestBuildArtifactOnlyIncludesSkills(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	g.Expect(os.MkdirAll(filepath.Join(dir, "skill-a"), 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "skill-a", "SKILL.md"), []byte("# a"), 0o644)).To(Succeed())
	g.Expect(os.MkdirAll(filepath.Join(dir, "skill-b"), 0o755)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "skill-b", "SKILL.md"), []byte("# b"), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("junk"), 0o644)).To(Succeed())

	data, err := BuildArtifact(dir, []string{"skill-a"})
	g.Expect(err).ToNot(HaveOccurred())

	entries := extractTarEntries(t, data)
	g.Expect(entries).To(ContainElement("skill-a/"))
	g.Expect(entries).To(ContainElement("skill-a/SKILL.md"))
	g.Expect(entries).ToNot(ContainElement("skill-b/"))
	g.Expect(entries).ToNot(ContainElement("stray.txt"))
}

func TestBuildArtifactEmpty(t *testing.T) {
	g := NewWithT(t)

	_, err := BuildArtifact(t.TempDir(), nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no skills to archive"))
}

func TestParseAnnotations(t *testing.T) {
	t.Run("valid key=value", func(t *testing.T) {
		g := NewWithT(t)

		annotations, err := ParseAnnotations([]string{
			"org.opencontainers.image.source=https://github.com/my-org/skills",
			"org.opencontainers.image.description=My skills",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(annotations).To(HaveKeyWithValue("org.opencontainers.image.source", "https://github.com/my-org/skills"))
		g.Expect(annotations).To(HaveKeyWithValue("org.opencontainers.image.description", "My skills"))
	})

	t.Run("value containing equals", func(t *testing.T) {
		g := NewWithT(t)

		annotations, err := ParseAnnotations([]string{
			"key=value=with=equals",
		})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(annotations).To(HaveKeyWithValue("key", "value=with=equals"))
	})

	t.Run("malformed input", func(t *testing.T) {
		g := NewWithT(t)

		_, err := ParseAnnotations([]string{"noequals"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key=value"))
	})
}

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "git protocol to https",
			input:    "git://github.com/my-org/repo.git",
			expected: "https://github.com/my-org/repo",
		},
		{
			name:     "strip .git suffix",
			input:    "https://github.com/my-org/repo.git",
			expected: "https://github.com/my-org/repo",
		},
		{
			name:     "SSH to https",
			input:    "git@github.com:my-org/repo.git",
			expected: "https://github.com/my-org/repo",
		},
		{
			name:     "already https",
			input:    "https://github.com/my-org/repo",
			expected: "https://github.com/my-org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(NormalizeGitURL(tt.input)).To(Equal(tt.expected))
		})
	}
}
