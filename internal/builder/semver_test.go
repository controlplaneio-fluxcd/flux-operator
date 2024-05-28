// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestMatchVersion(t *testing.T) {
	fluxDir := filepath.Join("testdata", "flux")
	tests := []struct {
		name     string
		exp      string
		expected string
	}{
		{name: "exact", exp: "v2.3.0", expected: "v2.3.0"},
		{name: "patch", exp: "2.2.x", expected: "v2.2.1"},
		{name: "minor", exp: "2.x", expected: "v2.3.0"},
		{name: "latest", exp: "*", expected: "v2.3.0"},
		{name: "invalid", exp: "3.x", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ver, err := MatchVersion(fluxDir, tt.exp)
			if tt.expected != "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ver).To(Equal(tt.expected))
			} else {
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}
