// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"bytes"
	"testing"
	"text/template"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"
)

func TestAutoUpdateTemplate_Render(t *testing.T) {
	g := NewWithT(t)

	artifactURL := "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
	ociRepository, err := renderAutoUpdateOCIRepository(artifactURL, "")
	g.Expect(err).NotTo(HaveOccurred())

	data := struct {
		Namespace     string
		ArtifactURL   string
		OCIRepository string
		Multitenant   bool
	}{
		Namespace:     "flux-system",
		ArtifactURL:   artifactURL,
		OCIRepository: yamlListItem(ociRepository),
		Multitenant:   false,
	}

	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	g.Expect(err).NotTo(HaveOccurred())

	result := buf.String()
	objects, err := ssautil.ReadObjects(bytes.NewReader([]byte(result)))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(result).To(ContainSubstring("kind: ResourceSet"))
	g.Expect(result).To(ContainSubstring("namespace: flux-system"))
	g.Expect(result).To(ContainSubstring("url: oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"))
	g.Expect(result).To(ContainSubstring("kind: OCIRepository"))
	g.Expect(result).NotTo(ContainSubstring("DEFAULT_SERVICE_ACCOUNT"))
}

func TestAutoUpdateTemplate_CustomOCIRepository(t *testing.T) {
	g := NewWithT(t)

	customOCIRepository := `apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: flux-operator
  namespace: flux-system
spec:
  interval: 5m
  url: oci://registry.internal/flux-operator-manifests
  ref:
    tag: latest
  insecure: true
`
	ociRepository, err := renderAutoUpdateOCIRepository("oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests", customOCIRepository)
	g.Expect(err).NotTo(HaveOccurred())

	data := struct {
		Namespace     string
		ArtifactURL   string
		OCIRepository string
		Multitenant   bool
	}{
		Namespace:     "flux-system",
		ArtifactURL:   "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests",
		OCIRepository: yamlListItem(ociRepository),
		Multitenant:   false,
	}

	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	g.Expect(err).NotTo(HaveOccurred())

	result := buf.String()
	objects, err := ssautil.ReadObjects(bytes.NewReader([]byte(result)))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(result).To(ContainSubstring("url: oci://registry.internal/flux-operator-manifests"))
	g.Expect(result).To(ContainSubstring("insecure: true"))
	g.Expect(result).NotTo(ContainSubstring("url: << inputs.url | quote >>"))
}

func TestAutoUpdateTemplate_Multitenant(t *testing.T) {
	g := NewWithT(t)

	artifactURL := "oci://ghcr.io/custom/manifests"
	ociRepository, err := renderAutoUpdateOCIRepository(artifactURL, "")
	g.Expect(err).NotTo(HaveOccurred())

	data := struct {
		Namespace     string
		ArtifactURL   string
		OCIRepository string
		Multitenant   bool
	}{
		Namespace:     "custom-ns",
		ArtifactURL:   artifactURL,
		OCIRepository: yamlListItem(ociRepository),
		Multitenant:   true,
	}

	tmpl, err := template.New("autoUpdate").Parse(autoUpdateTmpl)
	g.Expect(err).NotTo(HaveOccurred())

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	g.Expect(err).NotTo(HaveOccurred())

	result := buf.String()
	objects, err := ssautil.ReadObjects(bytes.NewReader([]byte(result)))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
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
			name:     "URL with port without tag",
			url:      "oci://localhost:5000/repo",
			expected: "oci://localhost:5000/repo",
		},
		{
			name:     "URL with digest",
			url:      "oci://ghcr.io/org/repo@sha256:abcdef",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL with tag and digest",
			url:      "oci://ghcr.io/org/repo:v1.0.0@sha256:abcdef",
			expected: "oci://ghcr.io/org/repo",
		},
		{
			name:     "URL with port, tag and digest",
			url:      "oci://localhost:5000/repo:v1.0.0@sha256:abcdef",
			expected: "oci://localhost:5000/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(artifactRepositoryURL(tt.url)).To(Equal(tt.expected))
		})
	}
}

func TestYAMLListItem(t *testing.T) {
	g := NewWithT(t)

	result := yamlListItem(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)

	g.Expect(result).To(Equal(`    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: test`))
}
