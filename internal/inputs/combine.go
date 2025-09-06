// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"fmt"
	"slices"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// Combined is a type for functions that expect a list of input sets that
// have already been combined according to an input strategy. The type is
// to emphasize this expectation for callers, e.g. builder.BuildResourceSet().
type Combined []combinedSet
type combinedSet map[string]any

type strategy interface {
	AddProvider(providerName string, providerInputs []map[string]any) error
	Combine() Combined
}

// ToCombined converts a slice of maps to the Combined type.
func ToCombined(inps []map[string]any) Combined {
	combined := make([]combinedSet, 0, len(inps))
	for _, inp := range inps {
		combined = append(combined, combinedSet(inp))
	}
	return combined
}

// Combine combines the inputs from the given ResourceSet and the given map of input
// providers according to the input strategy defined in the ResourceSet spec.
func Combine(rset *fluxcdv1.ResourceSet, providerMap map[ProviderKey]fluxcdv1.InputProvider) (Combined, error) {
	// Sort the provider keys to ensure consistent order.
	providerKeys := make([]ProviderKey, 0, len(providerMap))
	for k := range providerMap {
		providerKeys = append(providerKeys, k)
	}
	slices.SortFunc(providerKeys, compareProviderKeys)

	// Build a list of provider objects from the map with the RSET object as the first element.
	providers := make([]fluxcdv1.InputProvider, 0, len(providerKeys)+1)
	providers = append(providers, rset)
	for _, k := range providerKeys {
		providers = append(providers, providerMap[k])
	}

	// Determine the combination strategy.
	var combiner strategy
	switch strategy := rset.GetInputStrategy(); strategy {
	case fluxcdv1.InputStrategyFlatten:
		combiner = NewFlattener()
	case fluxcdv1.InputStrategyPermute:
		combiner = NewPermuter()
	default:
		return nil, fmt.Errorf("unknown input strategy: '%s'", strategy)
	}

	// Combine inputs based on the input strategy.
	for _, p := range providers {
		providerInputs, err := getFromProvider(p)
		if err != nil {
			return nil, fmt.Errorf("failed to get inputs from %s/%s: %w",
				p.GroupVersionKind().Kind, p.GetName(), err)
		}
		if err := combiner.AddProvider(p.GetName(), providerInputs); err != nil {
			return nil, fmt.Errorf("failed to get inputs from %s/%s: %w",
				p.GroupVersionKind().Kind, p.GetName(), err)
		}
	}

	return combiner.Combine(), nil
}
