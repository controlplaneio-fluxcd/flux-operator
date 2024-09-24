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

func TestBuildResourceGroup_Default(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourcegroup", "default.yaml")
	goldenFile := filepath.Join("testdata", "resourcegroup", "default.golden.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceGroup
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceGroup(rg.Spec.Resources, rg.GetInputs())
	g.Expect(err).ToNot(HaveOccurred())

	manifests, err := ssautil.ObjectsToYAML(objects)
	g.Expect(err).ToNot(HaveOccurred())

	//if shouldGenGolden() {
	err = os.WriteFile(goldenFile, []byte(manifests), 0644)
	g.Expect(err).NotTo(HaveOccurred())
	//}

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(manifests).To(Equal(string(goldenK)))
}

func TestBuildResourceGroup_Deduplication(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourcegroup", "dedup.yaml")
	goldenFile := filepath.Join("testdata", "resourcegroup", "dedup.golden.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceGroup
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceGroup(rg.Spec.Resources, rg.GetInputs())
	g.Expect(err).ToNot(HaveOccurred())

	manifests, err := ssautil.ObjectsToYAML(objects)
	g.Expect(err).ToNot(HaveOccurred())

	if shouldGenGolden() {
		err = os.WriteFile(goldenFile, []byte(manifests), 0644)
		g.Expect(err).NotTo(HaveOccurred())
	}

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(manifests).To(Equal(string(goldenK)))
}

func TestBuildResourceGroup_NoInputs(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourcegroup", "noinputs.yaml")
	goldenFile := filepath.Join("testdata", "resourcegroup", "noinputs.golden.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceGroup
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceGroup(rg.Spec.Resources, rg.GetInputs())
	g.Expect(err).ToNot(HaveOccurred())

	manifests, err := ssautil.ObjectsToYAML(objects)
	g.Expect(err).ToNot(HaveOccurred())

	if shouldGenGolden() {
		err = os.WriteFile(goldenFile, []byte(manifests), 0644)
		g.Expect(err).NotTo(HaveOccurred())
	}

	goldenK, err := os.ReadFile(goldenFile)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(manifests).To(Equal(string(goldenK)))
}

func TestBuildResourceGroup_Empty(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourcegroup", "empty.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceGroup
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceGroup(rg.Spec.Resources, rg.GetInputs())
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(BeEmpty())
}

func TestBuildResourceGroup_Error(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourcegroup", "error.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceGroup
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = BuildResourceGroup(rg.Spec.Resources, rg.GetInputs())
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to build resources[0]"))
}
