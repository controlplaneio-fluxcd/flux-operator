// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// IsCompatibleVersion checks if the version upgrade is compatible.
// It returns an error if a downgrade to a lower minor version is attempted.
func IsCompatibleVersion(fromVer, toVer string) error {
	if strings.Contains(fromVer, "@") {
		fromVer = strings.Split(fromVer, "@")[0]
	}
	from, err := semver.NewVersion(fromVer)
	if err != nil {
		return fmt.Errorf("from version '%s' parse error: %w", fromVer, err)
	}

	if strings.Contains(toVer, "@") {
		toVer = strings.Split(toVer, "@")[0]
	}
	to, err := semver.NewVersion(toVer)
	if err != nil {
		return fmt.Errorf("to version '%s' parse error: %w", toVer, err)
	}

	if to.Major() < from.Major() || to.Minor() < from.Minor() {
		return fmt.Errorf("downgrading from %s to %s is not supported, reinstall needed", fromVer, toVer)
	}

	return nil
}

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

// SupportsObjectLevelWorkloadFeature checks if the Flux version
// supports the object level workload feature.
func SupportsObjectLevelWorkloadFeature(fluxVersion string) (bool, error) {
	c, err := semver.NewConstraint("< 2.6.0")
	if err != nil {
		return false, fmt.Errorf("semver constraint parse error: %w", err)
	}

	version, err := semver.NewVersion(fluxVersion)
	if err != nil {
		return false, fmt.Errorf("version '%s' parse error: %w", fluxVersion, err)
	}

	if c.Check(version) {
		return false, nil
	}

	return true, nil
}

// EnableObjectLevelWorkloadFeature checks if the object level workload feature
// should be enabled based on the Flux version and the object level workload ID.
func EnableObjectLevelWorkloadFeature(fluxVersion string, objectLevelWorkloadID bool) (bool, error) {
	supported, err := SupportsObjectLevelWorkloadFeature(fluxVersion)
	if err != nil {
		return false, err
	}

	// If the Flux version is >= 2.6.0, we check if the object level workload ID is enabled.
	return supported && objectLevelWorkloadID, nil
}

// getSourceAPIVersion determines the API version of the source based on the provided Flux version.
func getSourceAPIVersion(fluxVersion string) (string, error) {
	sourceAPIVersion := "source.toolkit.fluxcd.io/v1beta2"

	version, err := semver.NewVersion(fluxVersion)
	if err != nil {
		return sourceAPIVersion, fmt.Errorf("version '%s' parse error: %w", fluxVersion, err)
	}

	c, err := semver.NewConstraint(">= 2.6.0")
	if err != nil {
		return sourceAPIVersion, fmt.Errorf("semver constraint parse error: %w", err)
	}

	if c.Check(version) {
		sourceAPIVersion = "source.toolkit.fluxcd.io/v1"
	}

	return sourceAPIVersion, nil
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
