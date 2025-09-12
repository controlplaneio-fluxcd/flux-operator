// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

const (
	maxPermutations = 10000
)

// Permuter accumulates a series of input provider objects and their
// respective list of exported inputs and generates the permutation
// L_1 x L_2 x ... x L_n, where L_i is the list of inputs exported
// by the i-th provider.
type Permuter struct {
	includeEmptyProviders bool
	scopedProviderInputs  [][]map[string]any // scopedProviderInputs[i] is the list of input sets for provider i
	permutations          Combined
	expectedPermutations  uint64
}

// PermuterOption is a functional option for configuring the Permuter.
type PermuterOption func(*Permuter)

// WithIncludeEmptyProviders configures the Permuter to include input providers
// that do not export any inputs when generating permutations. This allows
// generating an empty permutation when at least one provider has no inputs.
func WithIncludeEmptyProviders() PermuterOption {
	return func(p *Permuter) {
		p.includeEmptyProviders = true
	}
}

// NewPermuter returns a new initialized *Permuter.
func NewPermuter(opts ...PermuterOption) *Permuter {
	var p Permuter
	for _, opt := range opts {
		opt(&p)
	}
	return &p
}

// AddProvider accumulates the given input provider object
// and its exported inputs. If permutations have already been generated,
// the state is not mutated.
func (p *Permuter) AddProvider(providerName string, providerInputs []map[string]any) error {
	// If permutations have already been generated, do not mutate the state.
	if p.permutations != nil {
		return errors.New("permutations have already been generated, cannot add more inputs")
	}

	// If configured to skip empty providers and this one has no inputs,
	// do not add it to the list of provider inputs.
	if !p.includeEmptyProviders && len(providerInputs) == 0 {
		return nil
	}

	// Calculate the expected number of permutations.
	if len(p.scopedProviderInputs) == 0 {
		p.expectedPermutations = uint64(len(providerInputs))
	} else {
		p.expectedPermutations *= uint64(len(providerInputs))
	}

	// Error out if the expected number of permutations exceeds the maximum allowed.
	if p.expectedPermutations > maxPermutations {
		return fmt.Errorf("adding provider '%s' with %d inputs would exceed the maximum allowed permutations. max: %d, got: %d",
			providerName, len(providerInputs), maxPermutations, p.expectedPermutations)
	}

	// Error out for empty provider names.
	normalizedName := NormalizeKeyForTemplate(providerName)
	if normalizedName == "" {
		return fmt.Errorf("normalized provider name is empty: '%s'", providerName)
	}

	// Scope all input sets with the normalized provider name.
	scopedInputs := make([]map[string]any, 0, len(providerInputs))
	for _, inputSet := range providerInputs {
		scopedInputs = append(scopedInputs, map[string]any{
			string(normalizedName): inputSet,
		})
	}

	p.scopedProviderInputs = append(p.scopedProviderInputs, scopedInputs)
	return nil
}

// Combine generates the permutations of the inputs exported by
// the accumulated list of input provider objects and their
// exported inputs.
func (p *Permuter) Combine() Combined {
	if p.permutations == nil {
		p.permutations = Combined{}
		p.computePermutationsWithBacktracking()
	}
	return p.permutations
}

// computePermutationsWithBacktracking generates all the permutations
// of the accumulated provider inputs using a non-recursive backtracking
// algorithm and stores them in the p.permutations field. It's a bit
// more complicated than the recursive version, but it's worth it in
// case the number of providers is large (the recursion depth would be
// the number of providers).
func (p *Permuter) computePermutationsWithBacktracking() {
	// If no providers have been added, nothing to do.
	if len(p.scopedProviderInputs) == 0 {
		return
	}

	// If at least one provider has no inputs, nothing to do.
	for i := range p.scopedProviderInputs {
		if len(p.scopedProviderInputs[i]) == 0 {
			return
		}
	}

	// The algorithm is a loop driven by the state of two
	// variables: curProvider and selectedInputs.

	// curProvider is the index of the current provider.
	// The value of this variable goes back and forth in
	// the range [0, len(p.scopedProviderInputs)] while
	// the algorithm is running. Every time it reaches
	// len(p.scopedProviderInputs), a new permutation is
	// generated.
	var curProvider int

	// selectedInputs[i] holds the index of the currently selected
	// input set for the i-th provider.
	//
	// For i > 0, selectedInputs[i] goes back and forth in the range
	// [-1, len(p.scopedProviderInputs[i])], where -1 means no input set
	// is currently selected and len(p.scopedProviderInputs[i]) means
	// all input sets have been tried for the i-th provider in the
	// current permutation prefix.
	//
	// Only for i = 0, the value of selectedInputs[i] increases
	// monotonically from -1 to len(p.scopedProviderInputs[i]), at
	// which point the algorithm terminates.
	var selectedInputs = make([]int, len(p.scopedProviderInputs))
	for i := range selectedInputs {
		selectedInputs[i] = -1
	}

	for {
		// On every iteration we select the next input set for the current
		// provider and check the resulting state of the variables to act
		// accordingly.
		selectedInputs[curProvider]++

		// Backtrack to the previous provider while the current
		// provider has no more input sets to try.
		for selectedInputs[curProvider] == len(p.scopedProviderInputs[curProvider]) {
			if curProvider == 0 {
				// The value of selectedInputs[0] has reached
				// len(p.scopedProviderInputs[0]), so we are done.
				return
			}

			// Backtrack to the previous provider.
			selectedInputs[curProvider] = -1
			curProvider--

			// Make another attempt on selecting a next input set.
			selectedInputs[curProvider]++
		}

		// Now an input set has been selected for the
		// current provider. We can move to the next.
		curProvider++

		// If input sets have been selected for all providers,
		// collect the permutation.
		if curProvider == len(p.scopedProviderInputs) {
			p.collectPermutation(selectedInputs)

			// Backtrack to the last provider and let
			// the next iteration select the next
			// input set.
			curProvider--
		}
	}
}

// collectPermutation merges the selected input sets into a single permutation
// and appends it to the list of permutations.
func (p *Permuter) collectPermutation(selectedInputs []int) {
	perm := make(map[string]any)
	permIDComponents := make([]string, 0, len(selectedInputs))

	for providerIdx, inputSetIdx := range selectedInputs {
		// Merge the input set into the permutation.
		inputSet := p.scopedProviderInputs[providerIdx][inputSetIdx]
		maps.Copy(perm, inputSet)

		// Get the provider name (it's the only key in the input set
		// after we scoped it).
		var providerName string
		for k := range inputSet {
			providerName = k
			break
		}

		// Append the provider name and input set index to the permutation ID.
		permIDComponents = append(permIDComponents, fmt.Sprintf("%s=%d", providerName, inputSetIdx))
	}

	// Set the permutation ID.
	perm["id"] = ID(strings.Join(permIDComponents, "/"))

	// Append the permutation to the list.
	p.permutations = append(p.permutations, perm)
}
