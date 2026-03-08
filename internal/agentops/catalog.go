// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// DefaultSkillsDirName is the directory name for skills relative to .agents/.
	DefaultSkillsDirName = ".agents/skills"

	// CatalogFileName is the name of the catalog file.
	CatalogFileName = "catalog.yaml"
)

// DefaultSkillsDir resolves the skills directory to an absolute path
// relative to the current working directory. If the path exists, it
// verifies that it is a real directory (not a symlink).
func DefaultSkillsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	dir := filepath.Join(cwd, DefaultSkillsDirName)

	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return dir, nil
		}
		return "", fmt.Errorf("checking skills directory: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("skills directory %s is a symlink", dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("skills path %s is not a directory", dir)
	}

	return dir, nil
}

// LoadCatalog reads and unmarshals the catalog.yaml from the given directory.
// If the file does not exist, an empty catalog with TypeMeta is returned.
func LoadCatalog(dir string) (*fluxcdv1.AgentCatalog, error) {
	catalog := &fluxcdv1.AgentCatalog{}
	catalog.APIVersion = fluxcdv1.AgentGroupVersion.String()
	catalog.Kind = fluxcdv1.AgentCatalogKind

	path := filepath.Join(dir, CatalogFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return catalog, nil
		}
		return nil, fmt.Errorf("reading catalog: %w", err)
	}

	if err := yaml.Unmarshal(data, catalog); err != nil {
		return nil, fmt.Errorf("parsing catalog: %w", err)
	}

	return catalog, nil
}

// SaveCatalog atomically writes the catalog to catalog.yaml in the given directory.
func SaveCatalog(dir string, catalog *fluxcdv1.AgentCatalog) error {
	data, err := yaml.Marshal(catalog)
	if err != nil {
		return fmt.Errorf("marshaling catalog: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".catalog-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			return fmt.Errorf("writing catalog: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("writing catalog: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	dst := filepath.Join(dir, CatalogFileName)
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("renaming catalog: %w", err)
	}

	return nil
}

// CheckSkillConflicts checks if any skill name in newSkillNames is already
// installed by a different source in the catalog.
func CheckSkillConflicts(catalog *fluxcdv1.AgentCatalog, repo string, newSkillNames []string) error {
	newSet := make(map[string]struct{}, len(newSkillNames))
	for _, name := range newSkillNames {
		newSet[name] = struct{}{}
	}

	repoID := fluxcdv1.RepositoryID(repo)
	var conflicts []string
	for _, entry := range catalog.Status.Inventory {
		if entry.ID == repoID {
			continue
		}
		for _, skill := range entry.Skills {
			if _, ok := newSet[skill.Name]; ok {
				conflicts = append(conflicts, fmt.Sprintf("%q (from %s)", skill.Name, entry.URL))
			}
		}
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("skill name conflicts with other sources: %s", strings.Join(conflicts, ", "))
	}
	return nil
}
