// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/gomega"
)

func TestEncryptDecryptToken(t *testing.T) {
	g := NewWithT(t)
	// Generate test key pair
	publicKeySet, privateKeySet, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(publicKeySet.Keys).To(HaveLen(1))
	g.Expect(privateKeySet.Keys).To(HaveLen(1))

	t.Run("handles PAT payload", func(t *testing.T) {
		gt := NewWithT(t)

		testPayload := []byte("github_pat_11AEXAMPLE72qUp5aEXAMPLEbvt8LgEXAMPLEwDTR2EXAMPLEfKwjLEXAMPLE0TEXAMPLEU5EXAMPLEQzEXAMPLE")

		// Test successful encryption
		jweToken, err := EncryptTokenWithKeySet(testPayload, publicKeySet, "")
		gt.Expect(err).ToNot(HaveOccurred())
		gt.Expect(jweToken).ToNot(BeEmpty())

		// Test successful decryption
		decryptedPayload, err := DecryptTokenWithKeySet([]byte(jweToken), privateKeySet)
		gt.Expect(err).ToNot(HaveOccurred())
		gt.Expect(decryptedPayload).To(Equal(testPayload))
	})

	t.Run("handles large payload", func(t *testing.T) {
		gt := NewWithT(t)

		// Create a large payload
		largeData := strings.Repeat("test", 200000)
		largePayload := []byte(largeData)

		// Test successful encryption
		jweToken, err := EncryptTokenWithKeySet(largePayload, publicKeySet, "")
		gt.Expect(err).ToNot(HaveOccurred())
		gt.Expect(jweToken).ToNot(BeEmpty())

		// Log the JWE token size
		jweSize := len(jweToken)
		t.Logf("JWE token size: %.2f MiB", float64(jweSize)/(1024*1024))

		// Test successful decryption
		decryptedPayload, err := DecryptTokenWithKeySet([]byte(jweToken), privateKeySet)
		gt.Expect(err).ToNot(HaveOccurred())
		gt.Expect(decryptedPayload).To(Equal(largePayload))
	})
}

func TestEncryptTokenWithKeySetSpecificKID(t *testing.T) {
	g := NewWithT(t)

	// Generate test key pair
	publicKeySet, privateKeySet, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	kid := publicKeySet.Keys[0].KeyID
	testPayload := []byte("test payload with specific KID")

	// Test encryption with specific KID
	jweToken, err := EncryptTokenWithKeySet(testPayload, publicKeySet, kid)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(jweToken).ToNot(BeEmpty())

	// Test successful decryption
	decryptedPayload, err := DecryptTokenWithKeySet([]byte(jweToken), privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decryptedPayload).To(Equal(testPayload))
}

func TestEncryptTokenWithKeySetValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupKeySet func() (*jose.JSONWebKeySet, string)
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid key use",
			setupKeySet: func() (*jose.JSONWebKeySet, string) {
				publicKeySet, _, _ := NewEncryptionKeySet()
				key := publicKeySet.Keys[0]
				key.Use = "sig"
				return &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}}, ""
			},
			expectError: true,
			errorMsg:    ErrKeyNotFound.Error(),
		},
		{
			name: "invalid algorithm",
			setupKeySet: func() (*jose.JSONWebKeySet, string) {
				publicKeySet, _, _ := NewEncryptionKeySet()
				key := publicKeySet.Keys[0]
				key.Algorithm = "ES256"
				return &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}}, ""
			},
			expectError: true,
			errorMsg:    ErrKeyNotFound.Error(),
		},
		{
			name: "private key instead of public",
			setupKeySet: func() (*jose.JSONWebKeySet, string) {
				_, privateKeySet, _ := NewEncryptionKeySet()
				return privateKeySet, ""
			},
			expectError: true,
			errorMsg:    ErrKeyNotFound.Error(),
		},
		{
			name: "empty key set",
			setupKeySet: func() (*jose.JSONWebKeySet, string) {
				return &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{}}, ""
			},
			expectError: true,
			errorMsg:    ErrKeySetEmpty.Error(),
		},
		{
			name: "nil key set",
			setupKeySet: func() (*jose.JSONWebKeySet, string) {
				return nil, ""
			},
			expectError: true,
			errorMsg:    ErrKeySetEmpty.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			keySet, kid := tt.setupKeySet()
			testPayload := []byte("test payload")

			_, err := EncryptTokenWithKeySet(testPayload, keySet, kid)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.errorMsg))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestDecryptTokenWithKeySet(t *testing.T) {
	g := NewWithT(t)

	// Generate test key pair
	publicKeySet, privateKeySet, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	testPayload := []byte("test payload for key set decryption")

	// Encrypt with the public key
	jweToken, err := EncryptTokenWithKeySet(testPayload, publicKeySet, "")
	g.Expect(err).ToNot(HaveOccurred())

	// Test successful decryption with key set
	decryptedPayload, err := DecryptTokenWithKeySet([]byte(jweToken), privateKeySet)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decryptedPayload).To(Equal(testPayload))
}

func TestDecryptTokenWithKeySetValidation(t *testing.T) {
	tests := []struct {
		name      string
		setupData func() ([]byte, *jose.JSONWebKeySet)
		errorType error
	}{
		{
			name: "nil key set",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				publicKeySet, _, _ := NewEncryptionKeySet()
				jweToken, _ := EncryptTokenWithKeySet([]byte("test"), publicKeySet, "")
				return []byte(jweToken), nil
			},
			errorType: ErrKeySetEmpty,
		},
		{
			name: "empty key set",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				publicKeySet, _, _ := NewEncryptionKeySet()
				jweToken, _ := EncryptTokenWithKeySet([]byte("test"), publicKeySet, "")
				emptyKeySet := &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{}}
				return []byte(jweToken), emptyKeySet
			},
			errorType: ErrKeySetEmpty,
		},
		{
			name: "nil jwe data",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				_, privateKeySet, _ := NewEncryptionKeySet()
				return nil, privateKeySet
			},
			errorType: ErrPayloadEmpty,
		},
		{
			name: "empty jwe data",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				_, privateKeySet, _ := NewEncryptionKeySet()
				return []byte{}, privateKeySet
			},
			errorType: ErrPayloadEmpty,
		},
		{
			name: "invalid jwe token",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				_, privateKeySet, _ := NewEncryptionKeySet()
				return []byte("invalid.jwe.token"), privateKeySet
			},
			errorType: ErrParseJWE,
		},
		{
			name: "key set without matching KID",
			setupData: func() ([]byte, *jose.JSONWebKeySet) {
				// Create first key pair and encrypt
				publicKeySet1, _, _ := NewEncryptionKeySet()
				jweToken, _ := EncryptTokenWithKeySet([]byte("test"), publicKeySet1, "")

				// Create different key pair that won't match
				_, privateKeySet2, _ := NewEncryptionKeySet()
				return []byte(jweToken), privateKeySet2
			},
			errorType: ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			jweData, keySet := tt.setupData()
			_, err := DecryptTokenWithKeySet(jweData, keySet)

			g.Expect(err).To(HaveOccurred())
			g.Expect(errors.Is(err, tt.errorType)).To(BeTrue())
		})
	}
}

func TestDecryptTokenWithKeySetMultipleKeys(t *testing.T) {
	g := NewWithT(t)

	// Generate multiple key pairs
	publicKeySet1, privateKeySet1, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	publicKeySet2, privateKeySet2, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	// Create a key set with both private keys
	combinedKeySet := &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			privateKeySet1.Keys[0],
			privateKeySet2.Keys[0],
		},
	}

	testPayload := []byte("test payload with multiple keys")

	// Test decryption with first key
	jweToken1, err := EncryptTokenWithKeySet(testPayload, publicKeySet1, "")
	g.Expect(err).ToNot(HaveOccurred())

	decryptedPayload1, err := DecryptTokenWithKeySet([]byte(jweToken1), combinedKeySet)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decryptedPayload1).To(Equal(testPayload))

	// Test decryption with second key
	jweToken2, err := EncryptTokenWithKeySet(testPayload, publicKeySet2, "")
	g.Expect(err).ToNot(HaveOccurred())

	decryptedPayload2, err := DecryptTokenWithKeySet([]byte(jweToken2), combinedKeySet)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decryptedPayload2).To(Equal(testPayload))
}

func TestDecryptTokenWithKeySetInvalidKeys(t *testing.T) {
	g := NewWithT(t)

	// Generate valid key pair for encryption
	publicKeySet, _, err := NewEncryptionKeySet()
	g.Expect(err).ToNot(HaveOccurred())

	testPayload := []byte("test payload")
	jweToken, err := EncryptTokenWithKeySet(testPayload, publicKeySet, "")
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name        string
		setupKeySet func() *jose.JSONWebKeySet
		errorType   error
	}{
		{
			name: "key set with no KID",
			setupKeySet: func() *jose.JSONWebKeySet {
				_, privateKeySet, _ := NewEncryptionKeySet()
				key := privateKeySet.Keys[0]
				key.KeyID = "" // Remove KID
				return &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{key}}
			},
			errorType: ErrKeyNotFound, // Keys without KID are skipped, so no valid keys found
		},
		{
			name: "key set with public keys only",
			setupKeySet: func() *jose.JSONWebKeySet {
				publicKeySet, _, _ := NewEncryptionKeySet()
				return publicKeySet // Only public keys
			},
			errorType: ErrKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			keySet := tt.setupKeySet()
			_, err := DecryptTokenWithKeySet([]byte(jweToken), keySet)

			g.Expect(err).To(HaveOccurred())
			g.Expect(errors.Is(err, tt.errorType)).To(BeTrue())
		})
	}
}
