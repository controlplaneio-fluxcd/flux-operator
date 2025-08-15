// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestCheckOSMinimumVersion(t *testing.T) {
	tests := []struct {
		name       string
		osVersions map[string]int
		osRelease  string
		expected   bool
	}{
		{
			name:       "Debian Distroless OS - should pass",
			osVersions: map[string]int{"distroless": 12, "rhel": 8},
			osRelease: `PRETTY_NAME="Distroless"
NAME="Debian GNU/Linux"
ID="debian"
VERSION_ID="12"
VERSION="Debian GNU/Linux 12 (bookworm)"
HOME_URL="https://github.com/GoogleContainerTools/distroless"
SUPPORT_URL="https://github.com/GoogleContainerTools/distroless/blob/master/README.md"
BUG_REPORT_URL="https://github.com/GoogleContainerTools/distroless/issues/new"`,
			expected: true,
		},
		{
			name:       "Red Hat OS - should pass",
			osVersions: map[string]int{"Distroless": 12, "RHEL": 8},
			osRelease: `NAME="Red Hat Enterprise Linux"
VERSION="8.10 (Ootpa)"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="8.10"
PLATFORM_ID="platform:el8"
PRETTY_NAME="Red Hat Enterprise Linux 8.10 (Ootpa)"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:redhat:enterprise_linux:8::baseos"
HOME_URL="https://www.redhat.com/"
DOCUMENTATION_URL="https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/8"
BUG_REPORT_URL="https://issues.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 8"
REDHAT_BUGZILLA_PRODUCT_VERSION=8.10
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="8.10"`,
			expected: true,
		},
		{
			name:       "Alpine OS - should pass",
			osVersions: map[string]int{"alpine": 3},
			osRelease: `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.22.0
PRETTY_NAME="Alpine Linux v3.22"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues""`,
			expected: true,
		},
		{
			name:       "Unsupported OS - should fail",
			osVersions: map[string]int{"Distroless": 12, "rhel": 8},
			osRelease: `PRETTY_NAME="Ubuntu 20.04"
NAME="Ubuntu"
ID="ubuntu"
VERSION_ID="20.04"`,
			expected: false,
		},
		{
			name:       "Version too low for Distroless - should fail",
			osVersions: map[string]int{"Distroless": 12, "rhel": 8},
			osRelease: `PRETTY_NAME="Distroless"
ID="debian"
VERSION_ID="11"`,
			expected: false,
		},
		{
			name:       "Version too low for RHEL - should fail",
			osVersions: map[string]int{"Distroless": 12, "rhel": 8},
			osRelease: `ID="rhel"
PRETTY_NAME="Red Hat Enterprise Linux 7.9"
VERSION_ID="7"`,
			expected: false,
		},
		{
			name:       "Missing NAME - should fail",
			osVersions: map[string]int{"Distroless": 12, "rhel": 8},
			osRelease:  `VERSION_ID="7"`,
			expected:   false,
		},
		{
			name:       "Missing constraints - should fail",
			osVersions: nil,
			osRelease: `PRETTY_NAME="Alpine Linux v3.22"
ID=alpine
VERSION_ID=3.22.0`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			osInfo, err := ParseOSRelease(tt.osRelease)
			g.Expect(err).NotTo(HaveOccurred())

			result := CheckOSMinimumVersion(tt.osVersions, osInfo)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}
