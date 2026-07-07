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

// IsMinorUpgrade checks if the version upgrade is a minor version upgrade.
// It returns true if the major version is the same and the minor version is greater.
func IsMinorUpgrade(fromVer, toVer string) (bool, error) {
	if fromVer == "" {
		return false, nil
	}

	if strings.Contains(fromVer, "@") {
		fromVer = strings.Split(fromVer, "@")[0]
	}
	from, err := semver.NewVersion(fromVer)
	if err != nil {
		return false, fmt.Errorf("from version '%s' parse error: %w", fromVer, err)
	}

	if strings.Contains(toVer, "@") {
		toVer = strings.Split(toVer, "@")[0]
	}
	to, err := semver.NewVersion(toVer)
	if err != nil {
		return false, fmt.Errorf("to version '%s' parse error: %w", toVer, err)
	}

	if to.Major() == from.Major() && to.Minor() > from.Minor() {
		return true, nil
	}

	return false, nil
}

// MatchVersion returns the latest version dir path that matches the given semver range.
func MatchVersion(dataDir, semverRange string) (string, error) {
	matchingVersions, err := matchVersions(dataDir, semverRange, nil)
	if err != nil {
		return "", err
	}

	if len(matchingVersions) == 0 {
		return "", fmt.Errorf("no match found for semver: %s", semverRange)
	}

	sort.Sort(sort.Reverse(semver.Collection(matchingVersions)))
	return matchingVersions[0].Original(), nil
}

// MatchVersionWithEmbedded returns the latest version dir that matches the
// given semver range and belongs to a Flux minor version present in the
// embedded version directory bundled with the running operator image.
func MatchVersionWithEmbedded(dataDir, embeddedDataDir, semverRange string) (string, error) {
	embeddedMinorVersions, err := getMinorVersionSet(embeddedDataDir)
	if err != nil {
		return "", err
	}

	matchingVersions, err := matchVersions(dataDir, semverRange, embeddedMinorVersions)
	if err != nil {
		return "", err
	}

	if len(matchingVersions) == 0 {
		return "", fmt.Errorf("no match found for semver: %s in %s matching embedded minor versions in %s",
			semverRange, dataDir, embeddedDataDir)
	}

	sort.Sort(sort.Reverse(semver.Collection(matchingVersions)))
	return matchingVersions[0].Original(), nil
}

// matchVersions returns all version directories matching the semver range and optional allowed minor version set.
func matchVersions(dataDir, semverRange string, allowedMinorVersions map[string]struct{}) ([]*semver.Version, error) {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s directory not found", dataDir)
	}

	matches, err := filepath.Glob(dataDir + "/*")
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("semver '%s' parse error: %w", semverRange, err)
	}

	var matchingVersions []*semver.Version
	for _, t := range dirs {
		v, err := semver.NewVersion(t)
		if err != nil {
			continue
		}

		if constraint.Check(v) {
			if allowedMinorVersions != nil {
				if _, ok := allowedMinorVersions[majorMinor(v)]; !ok {
					continue
				}
			}
			matchingVersions = append(matchingVersions, v)
		}
	}

	return matchingVersions, nil
}

// getMinorVersionSet returns the normalized semver major.minor versions present in a directory.
func getMinorVersionSet(dataDir string) (map[string]struct{}, error) {
	versions, err := matchVersions(dataDir, "*", nil)
	if err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(versions))
	for _, v := range versions {
		result[majorMinor(v)] = struct{}{}
	}

	return result, nil
}

// majorMinor returns the major.minor segment of a semver version.
func majorMinor(v *semver.Version) string {
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
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

// CheckMinimumVersion checks if the given Flux version is greater than or equal to the minimum required version.
func CheckMinimumVersion(fluxVersion, minimumVersion string) error {
	version, err := semver.NewVersion(fluxVersion)
	if err != nil {
		return fmt.Errorf("failed to parse version '%s' as semver: %w", fluxVersion, err)
	}

	ok, err := checkVersionAgainstConstraint(version, fmt.Sprintf(">= %s", minimumVersion))
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	return fmt.Errorf("constraint '%s' >= '%s' not satisfied", fluxVersion, minimumVersion)
}

// checkVersionAgainstConstraint checks if a parsed semver matches the given constraint.
func checkVersionAgainstConstraint(version *semver.Version, constraint string) (bool, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return false, fmt.Errorf("failed to parse semver constraint '%s': %w", constraint, err)
	}
	return c.Check(version), nil
}

// ExtractVersionDigest extracts the version and digest from the given string.
// The input string is expected to be in one of the following formats:
// - "proto://host:port/org/app:vX.Y.Z@sha256:hex"
// - "host:port/org/app:vX.Y.Z@sha256:hex"
// - "host/org/app:vX.Y.Z"
// - "vX.Y.Z-RC.N@sha256:hex"
// - "vX.Y.Z"
// This function returns the version and optionally the digest as separate strings.
// An error is returned if the input string does not conform to the expected patterns.
func ExtractVersionDigest(input string) (string, string, error) {
	// Remove protocol prefix if present
	cleaned := input
	if strings.Contains(input, "://") {
		parts := strings.SplitN(input, "://", 2)
		if len(parts) == 2 {
			cleaned = parts[1]
		}
	}

	// Split by @ to separate version from digest
	parts := strings.Split(cleaned, "@")
	if len(parts) > 2 {
		return "", "", fmt.Errorf("invalid input format: %s", input)
	}

	versionPart := parts[0]
	digest := ""
	if len(parts) == 2 {
		digest = parts[1]
	}

	// Find the last occurrence of : to separate image from version
	lastColon := strings.LastIndex(versionPart, ":")
	if lastColon == -1 {
		// No version separator found, treat entire string as version
		return versionPart, digest, nil
	}

	version := versionPart[lastColon+1:]
	return version, digest, nil
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
