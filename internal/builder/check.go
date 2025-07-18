// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckPrerequisites checks if the build environment is compatible with the
// requirements of the Flux Operator running in-cluster.
func CheckPrerequisites(storagePath string) error {
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
		if err := CheckMinimumVersion(version, "2.2.0"); err != nil {
			return fmt.Errorf("version compatibility check failed: %w", err)
		}

		// Verify that the container OS matches the Flux Operator distribution variants:
		// - Google Distroless (min version Debian 12)
		// - Red Hat Universal Base Image (min version 8)
		osRelease, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return fmt.Errorf("failed to read /etc/os-release: %w", err)
		}
		osInfo, err := ParseOSRelease(string(osRelease))
		if err != nil {
			return err
		}
		if !CheckOSMinimumVersion(map[string]int{"distroless": 12, "rhel": 8}, osInfo) {
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

		// Remove quotes from value if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		result[key] = value
	}

	return result, nil
}

// CheckOSMinimumVersion checks if the OS info matches the minimum requirements.
func CheckOSMinimumVersion(osVersions map[string]int, osInfo map[string]string) bool {
	var matchedVersion int
	nameMatches := false

	// Check if any OS name matches and get the corresponding minimum version
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

	// Convert VERSION_ID to int for comparison
	var actualVersion int
	if _, err := fmt.Sscanf(versionID, "%d", &actualVersion); err != nil {
		return false
	}

	// Check if the minimum version is met
	return actualVersion >= matchedVersion
}
