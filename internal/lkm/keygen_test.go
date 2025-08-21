// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

func TestNewSigningKeySet(t *testing.T) {
	issuer := "test-issuer"

	t.Run("successfully generates key set pair", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, privateKeySet, err := NewSigningKeySet(issuer)

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
		publicKeySet1, privateKeySet1, err := NewSigningKeySet(issuer)
		g.Expect(err).ToNot(HaveOccurred())

		publicKeySet2, privateKeySet2, err := NewSigningKeySet(issuer)
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
		publicKeySet, privateKeySet, err := NewSigningKeySet("")

		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("issuer must be set before adding a private key"))
		g.Expect(publicKeySet).To(BeNil())
		g.Expect(privateKeySet).To(BeNil())
	})
}

func TestNewEncryptionKeySet(t *testing.T) {
	t.Run("successfully generates encryption key set pair", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, privateKeySet, err := NewEncryptionKeySet()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(publicKeySet).ToNot(BeNil())
		g.Expect(privateKeySet).ToNot(BeNil())

		// Verify public key set
		g.Expect(publicKeySet.Keys).To(HaveLen(1))
		g.Expect(publicKeySet.Keys[0].KeyID).ToNot(BeEmpty())
		g.Expect(publicKeySet.Keys[0].Algorithm).To(Equal(string(jose.ECDH_ES_A128KW)))
		g.Expect(publicKeySet.Keys[0].Use).To(Equal("enc"))
		g.Expect(publicKeySet.Keys[0].Valid()).To(BeTrue())

		// Verify the key ID is a valid UUID
		_, err = uuid.Parse(publicKeySet.Keys[0].KeyID)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify public key type
		_, ok := publicKeySet.Keys[0].Key.(*ecdsa.PublicKey)
		g.Expect(ok).To(BeTrue())
		g.Expect(publicKeySet.Keys[0].IsPublic()).To(BeTrue())

		// Verify private key set
		g.Expect(privateKeySet.Keys).To(HaveLen(1))
		g.Expect(privateKeySet.Keys[0].KeyID).To(Equal(publicKeySet.Keys[0].KeyID))
		g.Expect(privateKeySet.Keys[0].Algorithm).To(Equal(string(jose.ECDH_ES_A128KW)))
		g.Expect(privateKeySet.Keys[0].Use).To(Equal("enc"))
		g.Expect(privateKeySet.Keys[0].Valid()).To(BeTrue())

		// Verify private key type
		privateKey, ok := privateKeySet.Keys[0].Key.(*ecdsa.PrivateKey)
		g.Expect(ok).To(BeTrue())
		g.Expect(privateKeySet.Keys[0].IsPublic()).To(BeFalse())

		// Verify the keys are a valid pair
		publicKeyFromPrivate := &privateKey.PublicKey
		g.Expect(publicKeyFromPrivate).To(Equal(publicKeySet.Keys[0].Key))
	})

	t.Run("generates unique key IDs on multiple calls", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet1, privateKeySet1, err := NewEncryptionKeySet()
		g.Expect(err).ToNot(HaveOccurred())

		publicKeySet2, privateKeySet2, err := NewEncryptionKeySet()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify different key IDs
		g.Expect(publicKeySet1.Keys[0].KeyID).ToNot(Equal(publicKeySet2.Keys[0].KeyID))
		g.Expect(privateKeySet1.Keys[0].KeyID).ToNot(Equal(privateKeySet2.Keys[0].KeyID))

		// Verify different keys
		g.Expect(publicKeySet1.Keys[0].Key).ToNot(Equal(publicKeySet2.Keys[0].Key))
		g.Expect(privateKeySet1.Keys[0].Key).ToNot(Equal(privateKeySet2.Keys[0].Key))
	})

	t.Run("uses P-256 curve", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, privateKeySet, err := NewEncryptionKeySet()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify public key uses P-256 curve
		publicKey, ok := publicKeySet.Keys[0].Key.(*ecdsa.PublicKey)
		g.Expect(ok).To(BeTrue())
		g.Expect(publicKey.Curve.Params().Name).To(Equal("P-256"))

		// Verify private key uses P-256 curve
		privateKey, ok := privateKeySet.Keys[0].Key.(*ecdsa.PrivateKey)
		g.Expect(ok).To(BeTrue())
		g.Expect(privateKey.Curve.Params().Name).To(Equal("P-256"))
	})

	t.Run("write and read public key set", func(t *testing.T) {
		g := NewWithT(t)
		publicKeySet, _, err := NewEncryptionKeySet()
		g.Expect(err).ToNot(HaveOccurred())

		// Create temporary file
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "public.jwks")

		// Write key set to file
		err = WriteECDHKeySet(filename, publicKeySet)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify file exists and has correct permissions (0644 for public key)
		info, err := os.Stat(filename)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0644)))

		// Read key set from file
		readKeySet, err := ReadECDHKeySet(filename)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(readKeySet).ToNot(BeNil())

		// Verify the read key set matches the original
		g.Expect(readKeySet.Keys).To(HaveLen(1))
		g.Expect(readKeySet.Keys[0].KeyID).To(Equal(publicKeySet.Keys[0].KeyID))
		g.Expect(readKeySet.Keys[0].Algorithm).To(Equal(publicKeySet.Keys[0].Algorithm))
		g.Expect(readKeySet.Keys[0].Use).To(Equal(publicKeySet.Keys[0].Use))
		g.Expect(readKeySet.Keys[0].IsPublic()).To(BeTrue())
	})

	t.Run("write and read private key set", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKeySet, err := NewEncryptionKeySet()
		g.Expect(err).ToNot(HaveOccurred())

		// Create temporary file
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "private.jwks")

		// Write key set to file
		err = WriteECDHKeySet(filename, privateKeySet)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify file exists and has correct permissions (0600 for private key)
		info, err := os.Stat(filename)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))

		// Read key set from file
		readKeySet, err := ReadECDHKeySet(filename)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(readKeySet).ToNot(BeNil())

		// Verify the read key set matches the original
		g.Expect(readKeySet.Keys).To(HaveLen(1))
		g.Expect(readKeySet.Keys[0].KeyID).To(Equal(privateKeySet.Keys[0].KeyID))
		g.Expect(readKeySet.Keys[0].Algorithm).To(Equal(privateKeySet.Keys[0].Algorithm))
		g.Expect(readKeySet.Keys[0].Use).To(Equal(privateKeySet.Keys[0].Use))
		g.Expect(readKeySet.Keys[0].IsPublic()).To(BeFalse())
	})

	t.Run("write fails with empty key set", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "empty.jwks")

		// Test with nil key set
		err := WriteECDHKeySet(filename, nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key set is empty"))

		// Test with empty key set
		emptyKeySet := &jose.JSONWebKeySet{}
		err = WriteECDHKeySet(filename, emptyKeySet)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key set is empty"))
	})

	t.Run("write fails with invalid key", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "invalid.jwks")

		// Create key set with missing key ID
		invalidKeySet := &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:       &ecdsa.PrivateKey{},
					Algorithm: string(jose.ECDH_ES_A128KW),
					Use:       "enc",
				},
			},
		}

		err := WriteECDHKeySet(filename, invalidKeySet)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key ID is missing"))
	})

	t.Run("read fails with non-existent file", func(t *testing.T) {
		g := NewWithT(t)
		_, err := ReadECDHKeySet("/non/existent/file.jwks")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to read key set from file"))
	})

	t.Run("read fails with invalid JSON", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "invalid.jwks")

		// Write invalid JSON
		err := os.WriteFile(filename, []byte("invalid json"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = ReadECDHKeySet(filename)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to unmarshal key set"))
	})

	t.Run("read fails with empty key set", func(t *testing.T) {
		g := NewWithT(t)
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "empty.jwks")

		// Write empty key set
		emptyKeySet := &jose.JSONWebKeySet{}
		data, err := json.Marshal(emptyKeySet)
		g.Expect(err).ToNot(HaveOccurred())
		err = os.WriteFile(filename, data, 0644)
		g.Expect(err).ToNot(HaveOccurred())

		_, err = ReadECDHKeySet(filename)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key set is empty"))
	})
}
