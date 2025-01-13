// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"os"
	"path/filepath"
	"testing"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	v1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestBuildResourceSet(t *testing.T) {
	testdataRoot := filepath.Join("testdata", "resourceset")

	tests := []struct {
		name       string
		srcFile    string
		goldenFile string
	}{
		{
			name:       "default",
			srcFile:    filepath.Join(testdataRoot, "default.yaml"),
			goldenFile: filepath.Join(testdataRoot, "default.golden.yaml"),
		},
		{
			name:       "dedup",
			srcFile:    filepath.Join(testdataRoot, "dedup.yaml"),
			goldenFile: filepath.Join(testdataRoot, "dedup.golden.yaml"),
		},
		{
			name:       "exclude",
			srcFile:    filepath.Join(testdataRoot, "exclude.yaml"),
			goldenFile: filepath.Join(testdataRoot, "exclude.golden.yaml"),
		},
		{
			name:       "noinputs",
			srcFile:    filepath.Join(testdataRoot, "noinputs.yaml"),
			goldenFile: filepath.Join(testdataRoot, "noinputs.golden.yaml"),
		},
		{
			name:       "nestedinputs",
			srcFile:    filepath.Join(testdataRoot, "nestedinputs.yaml"),
			goldenFile: filepath.Join(testdataRoot, "nestedinputs.golden.yaml"),
		},
		{
			name:       "multi-doc-template",
			srcFile:    filepath.Join(testdataRoot, "multi-doc-template.yaml"),
			goldenFile: filepath.Join(testdataRoot, "multi-doc-template.golden.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			data, err := os.ReadFile(tt.srcFile)
			g.Expect(err).ToNot(HaveOccurred())

			var rg v1.ResourceSet
			err = yaml.Unmarshal(data, &rg)
			g.Expect(err).ToNot(HaveOccurred())

			objects, err := BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, rg.Spec.Inputs)
			g.Expect(err).ToNot(HaveOccurred())

			manifests, err := ssautil.ObjectsToYAML(objects)
			g.Expect(err).ToNot(HaveOccurred())

			if shouldGenGolden() {
				err = os.WriteFile(tt.goldenFile, []byte(manifests), 0644)
				g.Expect(err).NotTo(HaveOccurred())
			}

			goldenK, err := os.ReadFile(tt.goldenFile)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(manifests).To(Equal(string(goldenK)))
		})
	}
}

func TestBuildResourceSet_Empty(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourceset", "empty.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceSet
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, rg.Spec.Inputs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(BeEmpty())
}

func TestBuildResourceSet_Error(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourceset", "error.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceSet
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, rg.Spec.Inputs)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to build resources[0]"))
}
