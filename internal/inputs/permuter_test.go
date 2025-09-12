// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestNewPermuter(t *testing.T) {
	t.Run("includes empty providers when configured", func(t *testing.T) {
		g := NewWithT(t)

		p := inputs.NewPermuter(inputs.WithIncludeEmptyProviders())
		err := p.AddProvider("non-empty-provider", []map[string]any{{"key": "value"}})
		g.Expect(err).NotTo(HaveOccurred())
		err = p.AddProvider("empty-provider", []map[string]any{})
		g.Expect(err).NotTo(HaveOccurred())

		perms := p.Combine()
		g.Expect(perms).To(BeEmpty())
	})
}

func TestPermuter_AddProvider(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		providerNames        []string
		providerInputs       [][]map[string]any
		nonEmpty             bool
		genPerms             bool
		expectError          string
		expectedPermutations int
	}{
		{
			name:           "add provider after generating permutations",
			providerNames:  []string{"empty-provider"},
			providerInputs: [][]map[string]any{{}},
			genPerms:       true,
			expectError:    "permutations have already been generated, cannot add more inputs",
		},
		{
			name:                 "skips empty provider by default",
			providerNames:        []string{"empty-provider"},
			providerInputs:       [][]map[string]any{{}},
			nonEmpty:             true,
			expectedPermutations: 1,
		},
		{
			name:           "provider name is empty",
			providerNames:  []string{"-£%^$%&$%$£%$"},
			providerInputs: [][]map[string]any{{{}}},
			expectError:    "normalized provider name is empty: '-£%^$%&$%$£%$'",
		},
		{
			name: "too many permutations",
			providerNames: []string{
				"provider-1",
				"provider-2",
				"provider-3",
				"provider-4",
				"provider-5",
			},
			providerInputs: [][]map[string]any{
				{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
				{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
				{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
				{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
				{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
			},
			expectError: "adding provider 'provider-5' with 10 inputs would exceed the maximum allowed permutations. max: 10000, got: 100000",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			p := inputs.NewPermuter()

			if tt.nonEmpty {
				err := p.AddProvider("default", []map[string]any{{"key": "value"}})
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tt.genPerms {
				p.Combine()
			}

			var err error
			for i, providerName := range tt.providerNames {
				err = p.AddProvider(providerName, tt.providerInputs[i])
				if err != nil {
					break
				}
			}
			if tt.expectError != "" {
				g.Expect(err).To(MatchError(tt.expectError))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())

			perms := p.Combine()
			g.Expect(perms).To(HaveLen(tt.expectedPermutations))
		})
	}
}

func TestPermuter_Combine(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		providerNames        []string
		providerInputs       [][]map[string]any
		expectedPermutations inputs.Combined
	}{
		{
			name:          "one input set per provider",
			providerNames: []string{"provider-1", "provider-2"},
			providerInputs: [][]map[string]any{
				{
					{"key1": "value1"},
				},
				{
					{"key2": "value2"},
				},
			},
			expectedPermutations: inputs.Combined{{
				"id":         "2075461889",
				"provider_1": map[string]any{"key1": "value1"},
				"provider_2": map[string]any{"key2": "value2"},
			}},
		},
		{
			name:          "two input sets per provider",
			providerNames: []string{"provider-1", "provider-2"},
			providerInputs: [][]map[string]any{
				{
					{"key1": "value1"},
					{"key1": "value2"},
				},
				{
					{"key2": "value1"},
					{"key2": "value2"},
				},
			},
			expectedPermutations: inputs.Combined{
				{
					"id":         "2075461889",
					"provider_1": map[string]any{"key1": "value1"},
					"provider_2": map[string]any{"key2": "value1"},
				},
				{
					"id":         "2075527426",
					"provider_1": map[string]any{"key1": "value1"},
					"provider_2": map[string]any{"key2": "value2"},
				},
				{
					"id":         "2076379394",
					"provider_1": map[string]any{"key1": "value2"},
					"provider_2": map[string]any{"key2": "value1"},
				},
				{
					"id":         "2076444931",
					"provider_1": map[string]any{"key1": "value2"},
					"provider_2": map[string]any{"key2": "value2"},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			p := inputs.NewPermuter()

			for i, providerName := range tt.providerNames {
				err := p.AddProvider(providerName, tt.providerInputs[i])
				g.Expect(err).NotTo(HaveOccurred())
			}

			perms := p.Combine()
			g.Expect(perms).To(Equal(tt.expectedPermutations))
		})
	}
}
