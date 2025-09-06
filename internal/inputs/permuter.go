// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
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
	providerInputs        [][]map[string]any
	permutations          Combined
	expectedPermutations  uint64

	mu sync.Mutex

	// recursion state

	// selectedProviders counts how many provider input sets have been selected
	// in the current recursion branch.
	selectedProviders int

	// selectedInputs holds the index of the selected input set for each provider
	// in the current recursion branch.
	selectedInputs []int // provider index -> input set index
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

// normalizedObjectName is a type alias for string that represents
// a normalized name for Kubernetes objects.
type normalizedObjectName string

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
	p.mu.Lock()
	defer p.mu.Unlock()

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
	if len(p.providerInputs) == 0 {
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
	normalizedName := normalizeObjectName(providerName)
	if normalizedName == "" {
		return fmt.Errorf("normalized provider name is empty: '%s'", providerName)
	}

	// Scope all input sets with the normalized provider name.
	scopedInputs := make([]map[string]any, len(providerInputs))
	for i, inputSet := range providerInputs {
		scopedInputs[i] = map[string]any{
			string(normalizedName): inputSet,
		}
	}

	p.providerInputs = append(p.providerInputs, scopedInputs)
	return nil
}

// Combine generates the permutations of the inputs exported by
// the accumulated list of input provider objects and their
// exported inputs.
func (p *Permuter) Combine() Combined {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.permutations == nil {
		p.permutations = Combined{}
		p.selectedInputs = make([]int, len(p.providerInputs))
		p.computePermutations()
	}

	return p.permutations
}

// computePermutations generates the permutations of the inputs exported
// by the accumulated list of input provider objects and their exported
// inputs. It assumes the mutex is already locked.
func (p *Permuter) computePermutations() {
	// Recursive case: iterate over the inputs of the current provider,
	// select each input in turn, and recurse to select inputs from
	// the next provider.
	if p.selectedProviders < len(p.providerInputs) {
		curProvider := p.selectedProviders
		for inputSetIdx := range p.providerInputs[curProvider] {
			p.selectedInputs[curProvider] = inputSetIdx
			p.selectedProviders++
			p.computePermutations()
			p.selectedProviders--
		}
		return
	}

	// Base recursion case: if all providers have been selected,
	// combine the selected inputs into a single permutation and
	// save it.
	if p.selectedProviders == 0 {
		return
	}
	perm := make(map[string]any)
	permID := make([]string, 0, len(p.selectedInputs))
	for i, inputSetIdx := range p.selectedInputs {
		inputSet := p.providerInputs[i][inputSetIdx]
		maps.Copy(perm, inputSet)
		var providerName string
		for k := range inputSet {
			providerName = k
			break
		}
		permID = append(permID, fmt.Sprintf("%s=%d", providerName, inputSetIdx))
	}
	perm["id"] = ID(strings.Join(permID, "/"))
	p.permutations = append(p.permutations, perm)
}

// normalizeObjectName normalizes the given Kubernetes object name
// to a consistent format for use as a key in maps.
// We convert uppercase letters to lowercase, replace - with _,
// and remove any non-alphanumeric characters. Then we split the
// words by _ and join them back together with _, only the non-empty
// words.
func normalizeObjectName(name string) normalizedObjectName {

	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "-", "_")

	// Remove any non-alphanumeric characters except for _
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	name = sb.String()

	// Split by _ and join non-empty words with _
	var nonEmptyWords []string
	for word := range strings.SplitSeq(name, "_") {
		if word != "" {
			nonEmptyWords = append(nonEmptyWords, word)
		}
	}
	name = strings.Join(nonEmptyWords, "_")

	return normalizedObjectName(name)
}
