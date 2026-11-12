// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateAndApplySourceWatcherConfig(t *testing.T) {
	tests := []struct {
		name           string
		sourceWatcher  *fluxcdv1.SourceWatcher
		components     []string
		expectEnabled  bool
		expectErrorMsg string
	}{
		{
			name:          "nil configuration",
			sourceWatcher: nil,
		},
		{
			name:           "configuration without source-watcher",
			sourceWatcher:  &fluxcdv1.SourceWatcher{},
			components:     []string{"source-controller"},
			expectErrorMsg: "requires the source-watcher component to be enabled",
		},
		{
			name:          "namespace manager disabled",
			sourceWatcher: &fluxcdv1.SourceWatcher{NamespaceManager: false},
			components:    []string{"source-controller", "source-watcher"},
		},
		{
			name:          "namespace manager enabled",
			sourceWatcher: &fluxcdv1.SourceWatcher{NamespaceManager: true},
			components:    []string{"source-controller", "source-watcher"},
			expectEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			options := MakeDefaultOptions()
			options.Components = tt.components

			err := options.ValidateAndApplySourceWatcherConfig(tt.sourceWatcher)
			if tt.expectErrorMsg != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectErrorMsg))
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(options.SourceWatcherNamespaceManager).To(Equal(tt.expectEnabled))
		})
	}
}

func TestBuild_SourceWatcherNamespaceManager(t *testing.T) {
	g := NewWithT(t)
	const version = "v2.10.0"

	options := MakeDefaultOptions()
	options.Version = version
	options.Components = append([]string{}, AllComponents...)
	options.SourceWatcherNamespaceManager = true

	err := options.ValidateAndPatchComponents()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(options.HasSourceWatcher()).To(BeTrue())

	srcDir := filepath.Join("testdata", version)
	dstDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())

	ci, err := ExtractComponentImages(srcDir, options)
	g.Expect(err).ToNot(HaveOccurred())
	options.ComponentImages = ci

	result, err := Build(srcDir, dstDir, options)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Objects).NotTo(BeEmpty())

	var role, binding *unstructured.Unstructured
	for _, obj := range result.Objects {
		if !strings.Contains(obj.GetName(), "source-watcher-namespace-manager") {
			continue
		}
		switch obj.GetKind() {
		case "ClusterRole":
			role = obj
		case "ClusterRoleBinding":
			binding = obj
		}
	}

	g.Expect(role).NotTo(BeNil(), "source-watcher namespace manager ClusterRole should exist")
	rules, found, err := unstructured.NestedSlice(role.Object, "rules")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(rules).To(HaveLen(1))

	rule, ok := rules[0].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(rule["resources"]).To(ContainElement("namespaces"))
	g.Expect(rule["verbs"]).To(ContainElements(
		"get", "list", "watch", "create", "update", "patch", "delete"))

	g.Expect(binding).NotTo(BeNil(), "source-watcher namespace manager ClusterRoleBinding should exist")
	roleRefName, found, err := unstructured.NestedString(binding.Object, "roleRef", "name")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(roleRefName).To(Equal(role.GetName()))

	subjects, found, err := unstructured.NestedSlice(binding.Object, "subjects")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(subjects).To(HaveLen(1))

	subject, ok := subjects[0].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(subject["kind"]).To(Equal("ServiceAccount"))
	g.Expect(subject["name"]).To(Equal("source-watcher"))
	g.Expect(subject["namespace"]).To(Equal(options.Namespace))
}
