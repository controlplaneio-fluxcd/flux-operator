// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"bytes"
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
		{
			name:     "URL with port and no tag",
			url:      "oci://localhost:5000/repo",
			expected: "oci://localhost:5000/repo",
		},
		{
			name:     "URL with digest",
			url:      "oci://ghcr.io/org/repo@sha256:b5b2b2c507a0944348e0303114d8d93aaaa081732b86451d9bce1f432a537bc7",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL with tag and digest",
			url:      "oci://ghcr.io/org/repo:v1.0.0@sha256:b5b2b2c507a0944348e0303114d8d93aaaa081732b86451d9bce1f432a537bc7",
			expected: "oci://ghcr.io/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			artifactURL, err := artifactRepositoryURL(tt.url)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(artifactURL).To(Equal(tt.expected))
		})
	}
}

func TestAutoUpdateTemplate_TagStrippingInvalidURL(t *testing.T) {
	g := NewWithT(t)

	_, err := artifactRepositoryURL("oci://ghcr.io/org/repo@sha256:invalid")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("parsing artifact reference"))
}
