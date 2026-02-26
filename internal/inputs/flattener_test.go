// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestFlattener_EmptyProviders(t *testing.T) {
	g := NewWithT(t)

	f := inputs.NewFlattener()
	result := f.Combine()
	g.Expect(result).To(BeNil())
}

func TestFlattener_SingleProvider(t *testing.T) {
	g := NewWithT(t)

	f := inputs.NewFlattener()
	err := f.AddProvider("provider-1", []map[string]any{
		{"key": "val1"},
		{"key": "val2"},
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := f.Combine()
	g.Expect(result).To(HaveLen(2))
}

func TestFlattener_MultipleProviders(t *testing.T) {
	g := NewWithT(t)

	f := inputs.NewFlattener()
	err := f.AddProvider("provider-1", []map[string]any{
		{"key": "a"},
	})
	g.Expect(err).NotTo(HaveOccurred())

	err = f.AddProvider("provider-2", []map[string]any{
		{"key": "b"},
		{"key": "c"},
	})
	g.Expect(err).NotTo(HaveOccurred())

	result := f.Combine()
	g.Expect(result).To(HaveLen(3))
}

func TestFlattener_EmptyInputList(t *testing.T) {
	g := NewWithT(t)

	f := inputs.NewFlattener()
	err := f.AddProvider("empty", []map[string]any{})
	g.Expect(err).NotTo(HaveOccurred())

	result := f.Combine()
	g.Expect(result).To(BeNil())
}

func TestToCombined(t *testing.T) {
	g := NewWithT(t)

	raw := []map[string]any{
		{"a": "1"},
		{"b": "2"},
	}

	combined := inputs.ToCombined(raw)
	g.Expect(combined).To(HaveLen(2))
}

func TestToCombined_Empty(t *testing.T) {
	g := NewWithT(t)

	combined := inputs.ToCombined(nil)
	g.Expect(combined).To(BeEmpty())
}
