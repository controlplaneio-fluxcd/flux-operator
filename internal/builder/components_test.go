// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestValidateAndPatchComponents(t *testing.T) {
	tests := []struct {
		name                string
		version             string
		components          []string
		expectError         bool
		expectErrorMsg      string
		expectPatchContains string
	}{
		{
			name:           "empty components returns error",
			version:        "2.7.0",
			components:     []string{},
			expectError:    true,
			expectErrorMsg: "no components defined",
		},
		{
			name:           "invalid component returns error",
			version:        "2.7.0",
			components:     []string{"invalid-controller"},
			expectError:    true,
			expectErrorMsg: "invalid component: invalid-controller",
		},
		{
			name:                "default components with notification-controller adds patch",
			version:             "2.7.0",
			components:          DefaultComponents,
			expectPatchContains: "notification.toolkit.fluxcd.io",
		},
		{
			name:       "components without notification-controller omits patch",
			version:    "2.7.0",
			components: []string{"source-controller", "kustomize-controller", "helm-controller"},
		},
		{
			name:           "source-watcher with version < 2.7.0 returns error",
			version:        "2.6.0",
			components:     []string{"source-controller", "kustomize-controller", "source-watcher"},
			expectError:    true,
			expectErrorMsg: "source-watcher is only supported in Flux versions >= 2.7.0",
		},
		{
			name:                "source-watcher with version >= 2.7.0 adds feature gate",
			version:             "2.7.0",
			components:          []string{"source-controller", "kustomize-controller", "source-watcher"},
			expectPatchContains: "ExternalArtifact=true",
		},
		{
			name:           "invalid version returns error",
			version:        "not-a-version",
			components:     DefaultComponents,
			expectError:    true,
			expectErrorMsg: "failed to parse Flux version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			opts := MakeDefaultOptions()
			opts.Version = tt.version
			opts.Components = tt.components

			err := opts.ValidateAndPatchComponents()

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.expectErrorMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			if tt.expectPatchContains != "" {
				g.Expect(opts.Patches).To(ContainSubstring(tt.expectPatchContains))
			}
		})
	}
}

func TestValidateAndPatchComponents_NotificationCRDVersions(t *testing.T) {
	const (
		alerts    = "alerts.notification.toolkit.fluxcd.io"
		receivers = "receivers.notification.toolkit.fluxcd.io"
	)
	operatorKinds := []string{"FluxInstance", "ResourceSet", "ResourceSetInputProvider"}

	tests := []struct {
		version          string
		alertVersions    []int
		receiverVersions []int
	}{
		{version: "2.6.0", alertVersions: []int{0, 1, 2}, receiverVersions: []int{0, 1, 2}},
		{version: "2.8.0", alertVersions: []int{0, 1}, receiverVersions: []int{0, 1}},
		{version: "2.9.6", alertVersions: []int{0}, receiverVersions: []int{0}},
		{version: "2.10.0", alertVersions: []int{0, 1}, receiverVersions: []int{0}},
		{version: "2.11.0", alertVersions: []int{0, 1}, receiverVersions: []int{0}},
	}

	versionRe := regexp.MustCompile(`^/spec/versions/(\d+)/`)

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			g := NewWithT(t)

			opts := MakeDefaultOptions()
			opts.Version = tt.version
			opts.Components = DefaultComponents
			g.Expect(opts.ValidateAndPatchComponents()).To(Succeed())

			var patches []struct {
				Target struct {
					Name string `json:"name"`
				} `json:"target"`
				Patch string `json:"patch"`
			}
			g.Expect(yaml.Unmarshal([]byte(opts.Patches), &patches)).To(Succeed())

			// Map CRD name -> version index -> patched kinds.
			patched := map[string]map[int][]string{}
			for _, p := range patches {
				if p.Target.Name != alerts && p.Target.Name != receivers {
					continue
				}
				var ops []struct {
					Path  string `json:"path"`
					Value string `json:"value"`
				}
				g.Expect(yaml.Unmarshal([]byte(p.Patch), &ops)).To(Succeed())
				for _, op := range ops {
					m := versionRe.FindStringSubmatch(op.Path)
					g.Expect(m).To(HaveLen(2), "unexpected patch path %s", op.Path)
					idx, err := strconv.Atoi(m[1])
					g.Expect(err).NotTo(HaveOccurred())
					if patched[p.Target.Name] == nil {
						patched[p.Target.Name] = map[int][]string{}
					}
					patched[p.Target.Name][idx] = append(patched[p.Target.Name][idx], op.Value)
				}
			}

			assertVersions := func(crd string, expected []int) {
				got := make([]int, 0, len(patched[crd]))
				for idx, kinds := range patched[crd] {
					got = append(got, idx)
					g.Expect(kinds).To(ConsistOf(operatorKinds), "%s version %d", crd, idx)
				}
				slices.Sort(got)
				g.Expect(got).To(Equal(expected), crd)
			}
			assertVersions(alerts, tt.alertVersions)
			assertVersions(receivers, tt.receiverVersions)
		})
	}
}

func TestBuild_NotificationCRDKinds(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.10.0"
	options := MakeDefaultOptions()
	options.Version = version
	options.Components = DefaultComponents
	g.Expect(options.ValidateAndPatchComponents()).To(Succeed())

	srcDir := filepath.Join("testdata", version)
	dstDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).NotTo(HaveOccurred())
	options.ComponentImages = ci

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).NotTo(HaveOccurred())

	operatorKinds := []string{"FluxInstance", "ResourceSet", "ResourceSetInputProvider"}
	enumPaths := map[string][]string{
		"alerts.notification.toolkit.fluxcd.io":    {"spec", "eventSources", "items", "kind"},
		"receivers.notification.toolkit.fluxcd.io": {"spec", "resources", "items", "kind"},
	}

	found := map[string][]string{}
	for _, obj := range result.Objects {
		path, ok := enumPaths[obj.GetName()]
		if obj.GetKind() != "CustomResourceDefinition" || !ok {
			continue
		}
		versions, _, err := unstructured.NestedSlice(obj.Object, "spec", "versions")
		g.Expect(err).NotTo(HaveOccurred())
		for _, v := range versions {
			version := v.(map[string]any)
			schema := []string{"schema", "openAPIV3Schema", "properties", path[0], "properties", path[1], path[2], "properties", path[3], "enum"}
			enum, _, err := unstructured.NestedStringSlice(version, schema...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(enum).To(ContainElements(operatorKinds), "%s %s", obj.GetName(), version["name"])
			found[obj.GetName()] = append(found[obj.GetName()], version["name"].(string))
		}
	}

	g.Expect(found["alerts.notification.toolkit.fluxcd.io"]).To(ConsistOf("v1", "v1beta3"))
	g.Expect(found["receivers.notification.toolkit.fluxcd.io"]).To(ConsistOf("v1"))
}
