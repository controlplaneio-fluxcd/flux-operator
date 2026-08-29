// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package library

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{name: "repetitions", text: "Flux flux FLUX", want: []string{"flux", "flux", "flux"}},
		{name: "camelCase", text: "valuesFrom", want: []string{"valuesfrom", "values"}},
		{name: "multi-part camelCase", text: "ResourceSetInputProvider", want: []string{"resourcesetinputprovider", "resource", "set", "input", "provider"}},
		{name: "hyphen", text: "source-controller", want: []string{"source", "controller"}},
		{name: "dot", text: "spec.chart.spec.version", want: []string{"spec", "chart", "spec", "version"}},
		{name: "all punctuation", text: "source/controller_config:value", want: []string{"source", "controller", "config", "value"}},
		{name: "version and one character", text: "v1 v2beta3 x", want: []string{"v1", "v2beta3"}},
		{name: "stop words", text: "values from a ConfigMap", want: []string{"values", "configmap", "config", "map"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(Tokenize(tt.text)).To(Equal(tt.want))
		})
	}
}

func TestTokenizeGroups(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [][]string
	}{
		{name: "plain words", text: "Kustomization cel", want: [][]string{{"kustomization"}, {"cel"}}},
		{name: "camelCase keeps variants together", text: "Kustomization HelmRelease", want: [][]string{{"kustomization"}, {"helmrelease", "helm", "release"}}},
		{name: "filtered variants and words", text: "values from a ConfigMap x", want: [][]string{{"values"}, {"configmap", "config", "map"}}},
		{name: "only stop words", text: "the and from", want: [][]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(TokenizeGroups(tt.text)).To(Equal(tt.want))
		})
	}
}

func TestIsVersion(t *testing.T) {
	g := NewWithT(t)
	g.Expect(isVersion("v1")).To(BeTrue())
	g.Expect(isVersion("v2beta3")).To(BeTrue())
	g.Expect(isVersion("version")).To(BeFalse())
	g.Expect(isVersion("1")).To(BeFalse())
}
