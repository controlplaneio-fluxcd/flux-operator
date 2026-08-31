// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package docindex

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNormalizePath(t *testing.T) {
	g := NewWithT(t)
	for input, want := range map[string]string{
		"docs/crd/HelmRelease":                              "/docs/crd/helmrelease",
		" /docs/crd/helmrelease/ ":                          "/docs/crd/helmrelease",
		"https://fluxoperator.dev/docs/crd/HelmRelease.md/": "/docs/crd/helmrelease",
		"///docs/crd/helmrelease":                           "/docs/crd/helmrelease",
	} {
		g.Expect(NormalizePath(input)).To(Equal(want), input)
	}
}

func TestPathResolutionAndSectionPrefixes(t *testing.T) {
	g := NewWithT(t)
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	doc, found := idx.ResolveDoc("https://fluxoperator.dev/docs/crd/HelmRelease.md/")
	g.Expect(found).To(BeTrue())
	g.Expect(doc.Path).To(Equal("/docs/crd/helmrelease"))
	g.Expect(idx.IsSectionPrefix("/docs/crd")).To(BeTrue())
	g.Expect(idx.IsSectionPrefix("/docs/crds")).To(BeFalse())
	g.Expect(idx.SectionPrefixes()).To(Equal([]string{
		"/docs/guides", "/docs/instance", "/docs/resourcesets", "/docs/web-ui",
		"/docs/mcp", "/docs/crd", "/docs/controllers", "/docs/charts",
	}))
}

func TestClosePathMatches(t *testing.T) {
	g := NewWithT(t)
	idx, err := Get()
	g.Expect(err).ToNot(HaveOccurred())
	matches := idx.ClosePathMatches("helmreleas")
	g.Expect(matches).To(HaveLen(5))
	g.Expect(matches[0]).To(Equal("/docs/crd/helmrelease"))
	g.Expect(idx.ClosePathMatches(strings.Repeat("x", 500))).To(HaveLen(5))
	g.Expect(idx.ClosePathMatches("")).To(HaveLen(5))
	g.Expect(idx.ClosePathMatches("/")).To(HaveLen(5))
}

func TestClosePathMatchesRanksSubstringsFirst(t *testing.T) {
	g := NewWithT(t)
	idx := newTestIndex(
		[]Doc{
			{Path: "/docs/very/long/helm-guide"},
			{Path: "/docs/hel"},
		},
		nil,
	)
	g.Expect(idx.ClosePathMatches("helm")[0]).To(Equal("/docs/very/long/helm-guide"))
}
