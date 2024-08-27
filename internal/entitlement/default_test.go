// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package entitlement

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
)

func TestRegisterUsage(t *testing.T) {
	g := NewWithT(t)
	client := DefaultClient{Vendor: "testVendor"}
	ctx := context.Background()
	id := "testID"

	token, err := client.RegisterUsage(ctx, id)
	g.Expect(err).ToNot(HaveOccurred())

	expectedDigest := digest.FromString("testVendor-testID").Encoded()
	g.Expect(token).To(Equal(expectedDigest))
}

func TestVerify(t *testing.T) {
	g := NewWithT(t)
	client := DefaultClient{Vendor: "testVendor"}
	id := "testID"
	token := digest.FromString("testVendor-testID").Encoded()

	valid, err := client.Verify(token, id)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(valid).To(BeTrue())

	invalidToken := "invalidToken"
	valid, err = client.Verify(invalidToken, id)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(valid).To(BeFalse())
}

func TestGetVendor(t *testing.T) {
	g := NewWithT(t)
	client := DefaultClient{Vendor: "testVendor"}
	vendor := client.GetVendor()
	g.Expect(vendor).To(Equal("testVendor"))
}
