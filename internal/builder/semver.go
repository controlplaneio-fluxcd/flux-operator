// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// MatchVersion returns the latest version dir path that matches the given semver range.
func MatchVersion(dataDir, semverRange string) (string, error) {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%s directory not found", dataDir)
	}

	matches, err := filepath.Glob(dataDir + "/*")
	if err != nil {
		return "", err
	}

	var dirs []string
	for _, match := range matches {
		f, _ := os.Stat(match)
		if f.IsDir() {
			dirs = append(dirs, filepath.Base(match))
		}
	}

	constraint, err := semver.NewConstraint(semverRange)
	if err != nil {
		return "", fmt.Errorf("semver '%s' parse error: %w", semverRange, err)
	}

	var matchingVersions []*semver.Version
	for _, t := range dirs {
		v, err := semver.NewVersion(t)
		if err != nil {
			continue
		}

		if constraint.Check(v) {
			matchingVersions = append(matchingVersions, v)
		}
	}

	if len(matchingVersions) == 0 {
		return "", fmt.Errorf("no match found for semver: %s", semverRange)
	}

	sort.Sort(sort.Reverse(semver.Collection(matchingVersions)))
	return matchingVersions[0].Original(), nil
}

// MkdirTempAbs creates a tmp dir and returns the absolute path to the dir.
// This is required since certain OSes like MacOS create temporary files in
// e.g. `/private/var`, to which `/var` is a symlink.
func MkdirTempAbs(dir, pattern string) (string, error) {
	tmpDir, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: %w", err)
	}
	return tmpDir, nil
}
