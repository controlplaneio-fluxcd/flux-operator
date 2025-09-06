// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs

// Flattener is a strategy that flattens the inputs from multiple providers
// into a single list by concatenating them.
type Flattener struct {
	combined Combined
}

// NewFlattener returns a new initialized *Flattener.
func NewFlattener() *Flattener {
	return &Flattener{}
}

// AddProvider adds the inputs from a provider to the flattener.
func (f *Flattener) AddProvider(name string, inputs []map[string]any) error {
	f.combined = append(f.combined, ToCombined(inputs)...)
	return nil
}

// Combine returns the combined inputs as a single flattened list.
func (f *Flattener) Combine() Combined {
	return f.combined
}
