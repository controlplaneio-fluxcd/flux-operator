// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package agentops_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/agentops"
)

func TestNormalizeRepository(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "strips oci prefix", input: "oci://ghcr.io/org/repo", expect: "ghcr.io/org/repo"},
		{name: "strips trailing slash", input: "ghcr.io/org/repo/", expect: "ghcr.io/org/repo"},
		{name: "strips both", input: "oci://ghcr.io/org/repo/", expect: "ghcr.io/org/repo"},
		{name: "no change needed", input: "ghcr.io/org/repo", expect: "ghcr.io/org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(agentops.NormalizeRepository(tt.input)).To(Equal(tt.expect))
		})
	}
}

func TestIsGHCRHost(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{name: "ghcr.io host", input: "ghcr.io/org/repo", expect: true},
		{name: "non ghcr.io host", input: "docker.io/org/repo", expect: false},
		{name: "partial match", input: "notghcr.io/org/repo", expect: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(agentops.IsGitHubContainerRegistry(tt.input)).To(Equal(tt.expect))
		})
	}
}

func TestDeriveGHCROwner(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "extracts owner", input: "ghcr.io/myorg/repo", expect: "myorg"},
		{name: "extracts owner with nested path", input: "ghcr.io/myorg/subrepo/pkg", expect: "myorg"},
		{name: "no owner", input: "ghcr.io", expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(agentops.DeriveGitHubOwner(tt.input)).To(Equal(tt.expect))
		})
	}
}
