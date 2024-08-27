// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package entitlement

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewClient_DefaultVendor(t *testing.T) {
	g := NewWithT(t)

	// Unset the environment variable
	t.Setenv(MarketplaceTypeEnvKey, "")

	client, err := NewClient()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(client).ToNot(BeNil())
	g.Expect(client.GetVendor()).To(Equal(DefaultVendor))
}

func TestNewClient_UnsupportedVendor(t *testing.T) {
	g := NewWithT(t)

	// Set the environment variable to an unsupported value
	t.Setenv(MarketplaceTypeEnvKey, "unsupported")

	client, err := NewClient()
	g.Expect(err).To(HaveOccurred())
	g.Expect(client).To(BeNil())
}
