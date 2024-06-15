// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestBuild_ExtractImages(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	srcDir := filepath.Join("testdata", version)

	images, err := ExtractComponentImages(srcDir, MakeDefaultOptions())
	g.Expect(err).NotTo(HaveOccurred())

	t.Log(images)
	g.Expect(images).To(HaveLen(6))
	g.Expect(images).To(ContainElements(
		ComponentImage{
			Name:       "source-controller",
			Repository: "ghcr.io/fluxcd/source-controller",
			Tag:        "v1.3.0",
			Digest:     "",
		},
		ComponentImage{
			Name:       "kustomize-controller",
			Repository: "ghcr.io/fluxcd/kustomize-controller",
			Tag:        "v1.3.0",
			Digest:     "",
		},
	))
}

func TestBuild_ExtractImagesWithDigest(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.3.0"
	opts := MakeDefaultOptions()
	opts.Version = version
	opts.Registry = "ghcr.io/fluxcd"

	imagePath := filepath.Join("testdata", "flux-images")
	images, err := ExtractComponentImagesWithDigest(imagePath, opts)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(images).To(HaveLen(6))
	g.Expect(images).To(ContainElements(
		ComponentImage{
			Name:       "source-controller",
			Repository: "ghcr.io/fluxcd/source-controller",
			Tag:        "v1.3.0",
			Digest:     "sha256:161da425b16b64dda4b3cec2ba0f8d7442973aba29bb446db3b340626181a0bc",
		},
		ComponentImage{
			Name:       "kustomize-controller",
			Repository: "ghcr.io/fluxcd/kustomize-controller",
			Tag:        "v1.3.0",
			Digest:     "sha256:48a032574dd45c39750ba0f1488e6f1ae36756a38f40976a6b7a588d83acefc1",
		},
	))

	opts.Registry = "registry.local/fluxcd"
	_, err = ExtractComponentImagesWithDigest(imagePath, opts)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unsupported registry"))
}
