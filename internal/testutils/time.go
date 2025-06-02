// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package testutils

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// ParseTime parses a time string in RFC3339 format and returns a time.Time object.
func ParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	g := NewWithT(t)
	tm, err := time.Parse(time.RFC3339, s)
	g.Expect(err).NotTo(HaveOccurred())
	return tm
}
