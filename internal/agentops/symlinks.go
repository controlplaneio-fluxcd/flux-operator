// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SyncAgentSymlinks creates per-skill symlinks for each agent that uses a
// custom skills directory. TargetAgents using the default skills directory are
// silently skipped. The symlinks are relative so they remain valid across
// machines. Existing correct symlinks are left in place; wrong targets are
// replaced. Returns an error if a target path exists as a non-symlink.
func SyncAgentSymlinks(projectRoot string, agents []string, skillNames []string) error {
	for _, agentID := range agents {
		info := FindAgent(agentID)
		if info == nil || UsesDefaultSkillsDir(info) {
			continue
		}

		agentSkillsDir := filepath.Join(projectRoot, info.ProjectPath)
		if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
			return fmt.Errorf("creating agent skills directory %s: %w", agentSkillsDir, err)
		}

		targetPrefix := agentSymlinkPrefix(info.ProjectPath)
		for _, skillName := range skillNames {
			linkPath := filepath.Join(agentSkillsDir, skillName)
			target := filepath.Join(targetPrefix, skillName)

			fi, err := os.Lstat(linkPath)
			if err == nil {
				// Path exists.
				if fi.Mode()&os.ModeSymlink == 0 {
					return fmt.Errorf("cannot create symlink at %s: path exists and is not a symlink", linkPath)
				}
				// Check if the existing symlink points to the correct target.
				existing, readErr := os.Readlink(linkPath)
				if readErr == nil && existing == target {
					continue // already correct
				}
				// Wrong target — remove and recreate.
				if removeErr := os.Remove(linkPath); removeErr != nil {
					return fmt.Errorf("removing stale symlink %s: %w", linkPath, removeErr)
				}
			} else if !os.IsNotExist(err) {
				return fmt.Errorf("checking symlink path %s: %w", linkPath, err)
			}

			if err := os.Symlink(target, linkPath); err != nil {
				return fmt.Errorf("creating symlink %s -> %s: %w", linkPath, target, err)
			}
		}
	}
	return nil
}

// RemoveAgentSymlinks removes per-skill symlinks for each agent and cleans
// up empty agent skill directories and their parents.
func RemoveAgentSymlinks(projectRoot string, agents []string, skillNames []string) error {
	for _, agentID := range agents {
		info := FindAgent(agentID)
		if info == nil || UsesDefaultSkillsDir(info) {
			continue
		}

		agentSkillsDir := filepath.Join(projectRoot, info.ProjectPath)
		for _, skillName := range skillNames {
			linkPath := filepath.Join(agentSkillsDir, skillName)
			fi, err := os.Lstat(linkPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("checking symlink %s: %w", linkPath, err)
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(linkPath); err != nil {
					return fmt.Errorf("removing symlink %s: %w", linkPath, err)
				}
			}
		}

		// Clean up empty directories: agent skills dir, then parent.
		RemoveEmptyDir(agentSkillsDir)
		RemoveEmptyDir(filepath.Dir(agentSkillsDir))
	}
	return nil
}

// RemoveEmptyDir removes a directory if it is empty. Errors are silently ignored.
func RemoveEmptyDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

// agentSymlinkPrefix computes the relative path prefix from an agent's
// skills directory to the canonical skills directory.
// For example, ".claude/skills" -> "../../.agents/skills".
func agentSymlinkPrefix(agentProjectPath string) string {
	depth := len(strings.Split(filepath.ToSlash(agentProjectPath), "/"))
	parts := make([]string, 0, depth+1)
	for range depth {
		parts = append(parts, "..")
	}
	parts = append(parts, DefaultSkillsDirName)
	return filepath.Join(parts...)
}
