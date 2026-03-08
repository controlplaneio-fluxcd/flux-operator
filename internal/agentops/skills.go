// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

const (
	// SkillFileName is the name of the skill metadata file.
	SkillFileName = "SKILL.md"

	// maxSkillNameLength is the maximum length of a skill name.
	maxSkillNameLength = 64
)

var skillNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// ValidateSkillName validates a skill name per the Agent Skills specification.
func ValidateSkillName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("skill name must not be empty")
	}
	if len(name) > maxSkillNameLength {
		return fmt.Errorf("skill name %q exceeds maximum length of %d characters", name, maxSkillNameLength)
	}
	if !skillNameRegex.MatchString(name) {
		return fmt.Errorf("skill name %q is invalid: must be lowercase alphanumeric and hyphens, must not start or end with a hyphen", name)
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("skill name %q must not contain consecutive hyphens", name)
	}
	return nil
}

// skillFrontmatter holds the YAML frontmatter of a SKILL.md file.
type skillFrontmatter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DiscoverSkills walks top-level subdirectories of dir, finding those
// with a valid SKILL.md file. Returns the list of skill names.
func DiscoverSkills(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(dir, entry.Name(), SkillFileName)
		data, err := os.ReadFile(skillFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", skillFile, err)
		}

		fm, err := parseFrontmatter(data)
		if err != nil {
			return nil, fmt.Errorf("parsing frontmatter in %s: %w", skillFile, err)
		}

		if fm.Name == "" {
			return nil, fmt.Errorf("skill in %s is missing required 'name' field in frontmatter", entry.Name())
		}
		if fm.Description == "" {
			return nil, fmt.Errorf("skill %q in %s is missing required 'description' field in frontmatter", fm.Name, entry.Name())
		}
		if fm.Name != entry.Name() {
			return nil, fmt.Errorf("skill name %q in frontmatter does not match directory name %q", fm.Name, entry.Name())
		}
		if err := ValidateSkillName(fm.Name); err != nil {
			return nil, fmt.Errorf("skill in %s: %w", entry.Name(), err)
		}

		names = append(names, fm.Name)
	}

	return names, nil
}

// parseFrontmatter extracts YAML frontmatter from a markdown file.
func parseFrontmatter(data []byte) (*skillFrontmatter, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Look for opening ---
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return nil, fmt.Errorf("missing frontmatter delimiter")
	}

	var fmLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		fmLines = append(fmLines, line)
	}

	fm := &skillFrontmatter{}
	if err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), fm); err != nil {
		return nil, err
	}
	return fm, nil
}

// SkillDrift describes a skill that has drifted from its expected state.
type SkillDrift struct {
	// Name is the skill name.
	Name string

	// Reason is a human-readable description of the drift (e.g. "deleted", "modified").
	Reason string
}

// String returns a formatted drift description.
func (d SkillDrift) String() string {
	return fmt.Sprintf("%s (%s)", d.Name, d.Reason)
}

// CheckSkillIntegrity verifies that each installed skill directory exists and
// matches its stored checksum. Returns a list of drifted skills, or nil if all intact.
func CheckSkillIntegrity(skillsDir string, skills []fluxcdv1.AgentCatalogSkill) []SkillDrift {
	var drifted []SkillDrift
	for _, skill := range skills {
		skillDir := filepath.Join(skillsDir, skill.Name)
		info, err := os.Lstat(skillDir)
		if err != nil || !info.IsDir() {
			drifted = append(drifted, SkillDrift{Name: skill.Name, Reason: "deleted"})
			continue
		}
		checksum, err := HashSkillDir(skillDir)
		if err != nil || checksum != skill.Checksum {
			drifted = append(drifted, SkillDrift{Name: skill.Name, Reason: "modified"})
		}
	}
	return drifted
}

// SyncSkills computes checksums from a source directory, compares them
// against old checksums (from inventory) or installed checksums (when restoring),
// removes orphaned skills, and copies only changed skills to the install directory.
// Returns the list of changed skill names and the full skill metadata.
func SyncSkills(skillsDir, srcDir string, oldChecksums map[string]string, skillNames []string, restoring bool) ([]string, []fluxcdv1.AgentCatalogSkill, error) {
	skills := make([]fluxcdv1.AgentCatalogSkill, len(skillNames))
	var changedSkills []string
	newSkillSet := make(map[string]struct{}, len(skillNames))

	for j, name := range skillNames {
		newSkillSet[name] = struct{}{}
		newChecksum, err := HashSkillDir(filepath.Join(srcDir, name))
		if err != nil {
			return nil, nil, fmt.Errorf("computing checksum for skill %q: %w", name, err)
		}
		skills[j] = fluxcdv1.AgentCatalogSkill{Name: name, Checksum: newChecksum}

		// When restoring, compare against the installed dir to find
		// which skills actually need re-copying.
		if restoring {
			installedChecksum, _ := HashSkillDir(filepath.Join(skillsDir, name))
			if installedChecksum != newChecksum {
				changedSkills = append(changedSkills, name)
			}
		} else if oldChecksums[name] != newChecksum {
			changedSkills = append(changedSkills, name)
		}
	}

	// Remove orphaned skills (present in old inventory but not in new artifact).
	for oldName := range oldChecksums {
		if _, exists := newSkillSet[oldName]; !exists {
			if err := ValidateSkillName(oldName); err != nil {
				return nil, nil, fmt.Errorf("invalid old skill name: %w", err)
			}
			if err := os.RemoveAll(filepath.Join(skillsDir, oldName)); err != nil {
				return nil, nil, fmt.Errorf("removing orphaned skill %q: %w", oldName, err)
			}
		}
	}

	// Copy only the skills that need updating.
	for _, name := range changedSkills {
		dst := filepath.Join(skillsDir, name)
		if err := os.RemoveAll(dst); err != nil {
			return nil, nil, fmt.Errorf("removing old skill %q: %w", name, err)
		}
		if err := CopySkillDir(filepath.Join(srcDir, name), dst); err != nil {
			return nil, nil, fmt.Errorf("copying skill %q: %w", name, err)
		}
	}

	return changedSkills, skills, nil
}

// HashSkillDir computes the SHA-256 directory hash of a skill directory.
func HashSkillDir(dir string) (string, error) {
	checksum, _, err := lkm.HashDir(dir, "", nil, dirhash.Hash1)
	if err != nil {
		return "", fmt.Errorf("hashing skill directory %s: %w", dir, err)
	}
	return checksum, nil
}

// CopySkillDir performs a safe recursive directory copy. It skips symlinks
// and non-regular files to prevent symlink-based path traversal.
func CopySkillDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}

	return fs.WalkDir(os.DirFS(src), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, path)

		// Skip symlinks and non-regular files.
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		srcPath := filepath.Join(src, path)
		return copyFile(srcPath, dstPath, info.Mode())
	})
}

// copyFile copies a single file using streaming I/O.
func copyFile(src, dst string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		return fmt.Errorf("copying %s: %w", src, err)
	}

	return dstFile.Close()
}
