// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestID_Deterministic(t *testing.T) {
	g := NewWithT(t)

	id1 := inputs.ID("test-input")
	id2 := inputs.ID("test-input")
	g.Expect(id1).To(Equal(id2))
}

func TestID_DifferentInputs(t *testing.T) {
	g := NewWithT(t)

	id1 := inputs.ID("input-a")
	id2 := inputs.ID("input-b")
	g.Expect(id1).NotTo(Equal(id2))
}

func TestID_EmptyString(t *testing.T) {
	g := NewWithT(t)

	id := inputs.ID("")
	g.Expect(id).NotTo(BeEmpty())
}
