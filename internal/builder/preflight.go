// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PreflightOptions holds the configuration for checking prerequisites.
type PreflightOptions struct {
	minVersion     string
	containerOSMap map[string]int
}

// PreflightOption is a function that configures PreflightOptions.
type PreflightOption func(*PreflightOptions)

// WithMinVersion sets the minimum supported version.
func WithMinVersion(version string) PreflightOption {
	return func(opts *PreflightOptions) {
		opts.minVersion = version
	}
}

// WithContainerOS sets container OS requirements.
func WithContainerOS(osName string, minVersion int) PreflightOption {
	return func(opts *PreflightOptions) {
		if opts.containerOSMap == nil {
			opts.containerOSMap = make(map[string]int)
		}
		opts.containerOSMap[osName] = minVersion
	}
}

// PreflightChecks verifies if the build environment is compatible with the
// requirements of the Flux Operator running in-cluster.
func PreflightChecks(storagePath string, options ...PreflightOption) error {
	// Set up default options.
	opts := &PreflightOptions{
		minVersion: "2.2.0",
	}

	// Apply all option functions.
	for _, opt := range options {
		opt(opts)
	}

	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		// Verify the embedded storage path exists.
		if _, err := os.Stat(storagePath); os.IsNotExist(err) {
			return fmt.Errorf("storage path %s does not exist", storagePath)
		}

		// Verify the Flux version match minimum supported version.
		versionInfoPath := filepath.Join(storagePath, "flux-images", "VERSION")
		versionInfo, err := os.ReadFile(versionInfoPath)
		if err != nil {
			return fmt.Errorf("failed to read Flux version info from %s: %w", versionInfoPath, err)
		}

		// Check if the Flux version in storage is compatible with the operator.
		version := strings.TrimSpace(string(versionInfo))
		if err := CheckMinimumVersion(version, opts.minVersion); err != nil {
			return fmt.Errorf("version compatibility check failed: %w", err)
		}

		// Verify that the container OS matches the Flux Operator distros.
		osRelease, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return fmt.Errorf("failed to read /etc/os-release: %w", err)
		}
		osInfo, err := ParseOSRelease(string(osRelease))
		if err != nil {
			return err
		}
		if !CheckOSMinimumVersion(opts.containerOSMap, osInfo) {
			return fmt.Errorf("unsupported container OS version: %s", osInfo["VERSION"])
		}
	}

	return nil
}

// ParseOSRelease returns a map of key-value pairs representing the OS information.
func ParseOSRelease(content string) (map[string]string, error) {
	result := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes from value if present.
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		result[key] = value
	}

	if _, exists := result["VERSION_ID"]; !exists {
		return nil, fmt.Errorf("missing VERSION_ID in OS release information")
	}

	return result, nil
}

// CheckOSMinimumVersion checks if the OS info matches the minimum requirements.
func CheckOSMinimumVersion(osVersions map[string]int, osInfo map[string]string) bool {
	var matchedVersion int
	nameMatches := false

	// Check if any OS name matches and get the corresponding minimum version.
	for osName, minVersion := range osVersions {
		if strings.EqualFold(osInfo["PRETTY_NAME"], osName) || strings.EqualFold(osInfo["ID"], osName) {
			nameMatches = true
			matchedVersion = minVersion
			break
		}
	}

	if !nameMatches {
		return false
	}

	versionID := osInfo["VERSION_ID"]
	if versionID == "" {
		return false
	}

	// Extract major version from VERSION_ID.
	var actualVersion int
	if _, err := fmt.Sscanf(versionID, "%d", &actualVersion); err != nil {
		return false
	}

	// Check if the minimum version is met.
	return actualVersion >= matchedVersion
}
