// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

func TestNewKeySetPair(t *testing.T) {
	issuer := "test-issuer"

	t.Run("successfully generates key set pair", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, privateKeySet, err := NewKeySetPair(issuer)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(publicKeySet).ToNot(BeNil())
		g.Expect(privateKeySet).ToNot(BeNil())

		// Verify public key set
		g.Expect(publicKeySet.Issuer).To(BeEmpty())
		g.Expect(publicKeySet.Keys).To(HaveLen(1))
		g.Expect(publicKeySet.Keys[0].KeyID).ToNot(BeEmpty())
		g.Expect(publicKeySet.Keys[0].Algorithm).To(Equal("EdDSA"))
		g.Expect(publicKeySet.Keys[0].Use).To(Equal("sig"))

		// Verify the key ID is a valid UUID
		_, err = uuid.Parse(publicKeySet.Keys[0].KeyID)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify public key type
		_, ok := publicKeySet.Keys[0].Key.(ed25519.PublicKey)
		g.Expect(ok).To(BeTrue())

		// Verify private key set
		g.Expect(privateKeySet.Issuer).To(Equal(issuer))
		g.Expect(privateKeySet.Keys).To(HaveLen(1))
		g.Expect(privateKeySet.Keys[0].KeyID).To(Equal(publicKeySet.Keys[0].KeyID))
		g.Expect(privateKeySet.Keys[0].Algorithm).To(Equal("EdDSA"))
		g.Expect(privateKeySet.Keys[0].Use).To(Equal("sig"))

		// Verify private key type
		privateKey, ok := privateKeySet.Keys[0].Key.(ed25519.PrivateKey)
		g.Expect(ok).To(BeTrue())

		// Verify the keys are a valid pair
		publicKeyFromPrivate := privateKey.Public().(ed25519.PublicKey)
		g.Expect(publicKeyFromPrivate).To(Equal(publicKeySet.Keys[0].Key))
	})

	t.Run("generates unique key IDs on multiple calls", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet1, privateKeySet1, err := NewKeySetPair(issuer)
		g.Expect(err).ToNot(HaveOccurred())

		publicKeySet2, privateKeySet2, err := NewKeySetPair(issuer)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify different key IDs
		g.Expect(publicKeySet1.Keys[0].KeyID).ToNot(Equal(publicKeySet2.Keys[0].KeyID))
		g.Expect(privateKeySet1.Keys[0].KeyID).ToNot(Equal(privateKeySet2.Keys[0].KeyID))

		// Verify different keys
		g.Expect(publicKeySet1.Keys[0].Key).ToNot(Equal(publicKeySet2.Keys[0].Key))
		g.Expect(privateKeySet1.Keys[0].Key).ToNot(Equal(privateKeySet2.Keys[0].Key))
	})

	t.Run("fails with empty issuer", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, privateKeySet, err := NewKeySetPair("")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("issuer must be set before adding a private key"))
		g.Expect(publicKeySet).To(BeNil())
		g.Expect(privateKeySet).To(BeNil())
	})
}
