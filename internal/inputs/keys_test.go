// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package inputs_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

func TestNormalizeKeyForTemplate(t *testing.T) {
	for _, tt := range []struct {
		key         string
		expectedKey string
	}{
		{key: "My_ResourceSet", expectedKey: "my_resourceset"},
		{key: "My-ResourceSet", expectedKey: "my_resourceset"},
		{key: "My/ResourceSet", expectedKey: "my_resourceset"},
		{key: "My,ResourceSet", expectedKey: "my_resourceset"},
		{key: "My:ResourceSet", expectedKey: "my_resourceset"},
		{key: "My.ResourceSet", expectedKey: "my_resourceset"},
		{key: "My!ResourceSet", expectedKey: "my_resourceset"},
		{key: "My?ResourceSet", expectedKey: "my_resourceset"},
		{key: "My@ResourceSet", expectedKey: "my_resourceset"},
		{key: "My#ResourceSet", expectedKey: "my_resourceset"},
		{key: "My ResourceSet", expectedKey: "my_resourceset"},
		{key: "_a---b_-_", expectedKey: "a_b"},
		{key: "A0..B", expectedKey: "a0_b"},
		{key: "A  B", expectedKey: "a_b"},
		{key: "A@@B", expectedKey: "a_b"},
		{key: "A##B", expectedKey: "a_b"},
		{key: "__", expectedKey: ""},
	} {
		t.Run(tt.key, func(t *testing.T) {
			g := NewWithT(t)
			key := inputs.NormalizeKeyForTemplate(tt.key)
			g.Expect(string(key)).To(Equal(tt.expectedKey))
		})
	}
}
