// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	v1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestBuildResourceSet(t *testing.T) {
	testdataRoot := filepath.Join("testdata", "resourceset")

	tests := []struct {
		name       string
		srcFile    string
		goldenFile string
	}{
		{
			name:       "slugify",
			srcFile:    filepath.Join(testdataRoot, "slugify.yaml"),
			goldenFile: filepath.Join(testdataRoot, "slugify.golden.yaml"),
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

			inps, err := rg.GetInputs()
			g.Expect(err).ToNot(HaveOccurred())

			objects, err := BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, inputs.ToCombined(inps))
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

	inps, err := rg.GetInputs()
	g.Expect(err).ToNot(HaveOccurred())

	objects, err := BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, inputs.ToCombined(inps))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(objects).To(BeEmpty())
}

func TestBuildResourceSet_Error(t *testing.T) {
	testdataRoot := filepath.Join("testdata", "resourceset")

	tests := []struct {
		name     string
		srcFile  string
		matchErr string
	}{
		{
			name:     "fails with converting error",
			srcFile:  filepath.Join(testdataRoot, "invalid-output.yaml"),
			matchErr: `failed to build resources[0]: failed to read object: error converting YAML to JSON`,
		},
		{
			name:     "fails with missing input error",
			srcFile:  filepath.Join(testdataRoot, "missing-inputs.yaml"),
			matchErr: `<inputs>: map has no entry for key "semver"`,
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

			inps, err := rg.GetInputs()
			g.Expect(err).ToNot(HaveOccurred())

			_, err = BuildResourceSet(rg.Spec.ResourcesTemplate, rg.Spec.Resources, inputs.ToCombined(inps))
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tt.matchErr))
		})
	}
}

func TestBuildResourceSetSteps(t *testing.T) {
	testdataRoot := filepath.Join("testdata", "resourceset")

	tests := []struct {
		name       string
		srcFile    string
		goldenFile string
	}{
		{
			name:       "steps",
			srcFile:    filepath.Join(testdataRoot, "steps.yaml"),
			goldenFile: filepath.Join(testdataRoot, "steps.golden.yaml"),
		},
		{
			name:       "steps-noinputs",
			srcFile:    filepath.Join(testdataRoot, "steps-noinputs.yaml"),
			goldenFile: filepath.Join(testdataRoot, "steps-noinputs.golden.yaml"),
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

			inps, err := rg.GetInputs()
			g.Expect(err).ToNot(HaveOccurred())

			results, err := BuildResourceSetSteps(rg.Spec.Steps, inputs.ToCombined(inps))
			g.Expect(err).ToNot(HaveOccurred())

			// verify steps that build zero objects are kept in the
			// results to preserve the order and names of the steps
			g.Expect(results).To(HaveLen(len(rg.Spec.Steps)))
			for i, step := range rg.Spec.Steps {
				g.Expect(results[i].Name).To(Equal(step.Name))
			}

			sb := &strings.Builder{}
			for _, result := range results {
				fmt.Fprintf(sb, "# step: %s\n", result.Name)
				manifests, err := ssautil.ObjectsToYAML(result.Objects)
				g.Expect(err).ToNot(HaveOccurred())
				sb.WriteString(manifests)
			}

			if shouldGenGolden() {
				err = os.WriteFile(tt.goldenFile, []byte(sb.String()), 0644)
				g.Expect(err).NotTo(HaveOccurred())
			}

			goldenK, err := os.ReadFile(tt.goldenFile)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(sb.String()).To(Equal(string(goldenK)))
		})
	}
}

func TestBuildResourceSetSteps_Error(t *testing.T) {
	g := NewWithT(t)

	srcFile := filepath.Join("testdata", "resourceset", "steps-duplicate.yaml")

	data, err := os.ReadFile(srcFile)
	g.Expect(err).ToNot(HaveOccurred())

	var rg v1.ResourceSet
	err = yaml.Unmarshal(data, &rg)
	g.Expect(err).ToNot(HaveOccurred())

	inps, err := rg.GetInputs()
	g.Expect(err).ToNot(HaveOccurred())

	_, err = BuildResourceSetSteps(rg.Spec.Steps, inputs.ToCombined(inps))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(`duplicate resource ConfigMap/apps/app-config in step "deploy", already defined in step "pre-deploy"`))
}

func TestBuildResourceSetSteps_CrossVersionDuplicates(t *testing.T) {
	g := NewWithT(t)

	// the same cluster object referenced with different versions of the
	// same group must be detected as a cross-step duplicate, as the
	// object identity on the cluster does not include the version
	data := `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: apps
spec:
  steps:
    - name: pre-deploy
      resources:
        - apiVersion: helm.toolkit.fluxcd.io/v2beta2
          kind: HelmRelease
          metadata:
            name: app
            namespace: apps
    - name: deploy
      resources:
        - apiVersion: helm.toolkit.fluxcd.io/v2
          kind: HelmRelease
          metadata:
            name: app
            namespace: apps
`
	var rg v1.ResourceSet
	err := yaml.Unmarshal([]byte(data), &rg)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = BuildResourceSetSteps(rg.Spec.Steps, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(`duplicate resource HelmRelease/apps/app in step "deploy", already defined in step "pre-deploy"`))
}

func TestBuildResourceSetSteps_MissingResources(t *testing.T) {
	g := NewWithT(t)

	steps := []v1.ResourceSetStep{
		{
			Name: "pre-deploy",
			ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: apps
`,
		},
		{
			Name: "deploy",
		},
	}

	// the step fields are validated independently of rendering so that
	// an invalid spec is rejected even when no resources are built
	err := ValidateResourceSetSpec(v1.ResourceSetSpec{Steps: steps})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(`step "deploy": at least one of resources or resourcesTemplate must be set`))

	_, err = BuildResourceSetFromSpec(v1.ResourceSetSpec{Steps: steps}, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(`step "deploy": at least one of resources or resourcesTemplate must be set`))
}

func TestBuildResourceSetSteps_IntraStepDuplicates(t *testing.T) {
	g := NewWithT(t)

	// static resources built without inputs are not deduplicated by
	// BuildResourceSet, the duplicates within the same step must follow
	// these semantics instead of triggering the cross-step duplicate error
	data := `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: apps
spec:
  steps:
    - name: pre-deploy
      resources:
        - apiVersion: v1
          kind: ConfigMap
          metadata:
            name: app-config
            namespace: apps
        - apiVersion: v1
          kind: ConfigMap
          metadata:
            name: app-config
            namespace: apps
`
	var rg v1.ResourceSet
	err := yaml.Unmarshal([]byte(data), &rg)
	g.Expect(err).ToNot(HaveOccurred())

	results, err := BuildResourceSetSteps(rg.Spec.Steps, nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(results).To(HaveLen(1))
	g.Expect(results[0].Objects).To(HaveLen(2))
}
