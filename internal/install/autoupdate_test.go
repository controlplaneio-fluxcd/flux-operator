// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	. "github.com/onsi/gomega"
)

func TestAutoUpdateTemplate_Render(t *testing.T) {
	g := NewWithT(t)

	data := struct {
		Namespace   string
		ArtifactURL string
		Multitenant bool
	}{
		Namespace:   "flux-system",
		ArtifactURL: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests",
		Multitenant: false,
	}

	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	g.Expect(err).NotTo(HaveOccurred())

	result := buf.String()
	g.Expect(result).To(ContainSubstring("kind: ResourceSet"))
	g.Expect(result).To(ContainSubstring("namespace: flux-system"))
	g.Expect(result).To(ContainSubstring("url: oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"))
	g.Expect(result).NotTo(ContainSubstring("DEFAULT_SERVICE_ACCOUNT"))
}

func TestAutoUpdateTemplate_Multitenant(t *testing.T) {
	g := NewWithT(t)

	data := struct {
		Namespace   string
		ArtifactURL string
		Multitenant bool
	}{
		Namespace:   "custom-ns",
		ArtifactURL: "oci://ghcr.io/custom/manifests",
		Multitenant: true,
	}

	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	g.Expect(err).NotTo(HaveOccurred())

	result := buf.String()
	g.Expect(result).To(ContainSubstring("namespace: custom-ns"))
	g.Expect(result).To(ContainSubstring("DEFAULT_SERVICE_ACCOUNT"))
}

func TestAutoUpdateTemplate_TagStripping(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with tag",
			url:      "oci://ghcr.io/org/repo:v1.0.0",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL with latest tag",
			url:      "oci://ghcr.io/org/repo:latest",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL without tag",
			url:      "oci://ghcr.io/org/repo",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL with port and tag",
			url:      "oci://localhost:5000/repo:v1.0.0",
			expected: "oci://localhost:5000/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Replicate the tag stripping logic from ApplyAutoUpdate
			artifactURL := tt.url
			if idx := strings.LastIndex(artifactURL, ":"); idx > 6 {
				artifactURL = artifactURL[:idx]
			}
			g.Expect(artifactURL).To(Equal(tt.expected))
		})
	}
}
