// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"testing"

	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigMeta_GetReference(t *testing.T) {
	g := NewWithT(t)
	meta := ConfigMeta{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TestKind",
			APIVersion: "mcp.fluxcd.controlplane.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
	}
	ref := meta.GetReference()
	g.Expect(ref.Kind).To(Equal("TestKind"))
	g.Expect(ref.Name).To(Equal("test-name"))
}
