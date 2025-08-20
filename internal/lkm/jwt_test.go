// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestVerifyToken(t *testing.T) {
	t.Run("successfully verifies valid token with JWKs", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, privateKey := genTestKeys(t)
		payload := `{"iss":"test-issuer","sub":"test-subject","aud":"test-audience","iat":1735686000}`

		// Create JWT token
		token, err := GenerateSignedToken([]byte(payload), privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		// Create JWKs containing the public key
		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(publicKey.Key, publicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the token
		verifiedPayload, err := VerifySignedToken([]byte(token), jwksData)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(verifiedPayload)).To(Equal(payload))
	})

	t.Run("fails with invalid JWT token", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, _ := genTestKeys(t)

		keySet := NewPublicKeySet()
		err := keySet.AddPublicKey(publicKey.Key, publicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		_, err = VerifySignedToken([]byte("invalid-token"), jwksData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with token missing signatures", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, _ := genTestKeys(t)

		keySet := NewPublicKeySet()
		err := keySet.AddPublicKey(publicKey.Key, publicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		// Create a malformed JWT token (just base64 header and payload, no signature)
		malformedToken := "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJ0ZXN0LWlzc3VlciJ9"

		_, err = VerifySignedToken([]byte(malformedToken), jwksData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with token missing kid header", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, privateKey := genTestKeys(t)

		// Create a private key without KeyID to generate token without kid header
		privateKeyNoKID := &EdPrivateKey{
			Key:    privateKey.Key,
			KeyID:  "", // Empty KeyID
			Issuer: privateKey.Issuer,
		}

		payload := `{"iss":"test-issuer","sub":"test-subject"}`
		token, err := GenerateSignedToken([]byte(payload), privateKeyNoKID)
		g.Expect(err).ToNot(HaveOccurred())

		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(publicKey.Key, publicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		_, err = VerifySignedToken([]byte(token), jwksData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrKIDNotFoundInJWT)).To(BeTrue())
	})

	t.Run("fails with kid not found in JWKs", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, privateKey := genTestKeys(t)
		payload := `{"iss":"test-issuer","sub":"test-subject"}`

		// Create JWT token
		token, err := GenerateSignedToken([]byte(payload), privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		// Create JWKs with different key ID
		differentPublicKey := &EdPublicKey{
			Key:   publicKey.Key,
			KeyID: "different-key-id", // Different key ID
		}
		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(differentPublicKey.Key, differentPublicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		_, err = VerifySignedToken([]byte(token), jwksData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrKIDNotFoundInJWKs)).To(BeTrue())
	})

	t.Run("fails with wrong public key", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)
		wrongPublicKey, _ := genTestKeys(t) // Generate a different key pair
		payload := `{"iss":"test-issuer","sub":"test-subject"}`

		// Create JWT token with one private key
		token, err := GenerateSignedToken([]byte(payload), privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		// Create JWKs with the wrong public key but same KeyID
		wrongPublicKey.KeyID = privateKey.KeyID
		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(wrongPublicKey.Key, wrongPublicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		_, err = VerifySignedToken([]byte(token), jwksData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrVerifySig)).To(BeTrue())
	})

	t.Run("fails with invalid JWKs data", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)
		payload := `{"iss":"test-issuer","sub":"test-subject"}`

		token, err := GenerateSignedToken([]byte(payload), privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		invalidJWKs := []byte(`{"invalid": "jwks"}`)
		_, err = VerifySignedToken([]byte(token), invalidJWKs)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrKIDNotFoundInJWKs)).To(BeTrue())
	})
}

func TestGenerateJWT(t *testing.T) {
	t.Run("successfully generates JWT token", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)
		payload := []byte(`{"iss":"test-issuer","sub":"test-subject","aud":"test-audience"}`)

		token, err := GenerateSignedToken(payload, privateKey)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())

		// JWT should have 3 parts separated by dots
		g.Expect(strings.Count(token, ".")).To(Equal(2))

		// Verify we can extract the key ID
		keyID, err := GetKeyIDFromToken([]byte(token))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keyID).To(Equal(privateKey.KeyID))
	})

	t.Run("fails with nil private key", func(t *testing.T) {
		g := NewWithT(t)
		payload := []byte(`{"iss":"test-issuer"}`)

		token, err := GenerateSignedToken(payload, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(errors.Is(err, ErrPrivateKeyRequired)).To(BeTrue())
	})

	t.Run("handles empty payload", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)
		payload := []byte("")

		token, err := GenerateSignedToken(payload, privateKey)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(strings.Count(token, ".")).To(Equal(2))
	})

	t.Run("handles large payload", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)

		// Create a large JSON payload
		largeData := strings.Repeat("test", 10000)
		payload := []byte(`{"iss":"test-issuer","data":"` + largeData + `"}`)

		token, err := GenerateSignedToken(payload, privateKey)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(strings.Count(token, ".")).To(Equal(2))
	})
}

func TestGetKeyIDFromToken(t *testing.T) {
	t.Run("extracts key ID from valid token", func(t *testing.T) {
		g := NewWithT(t)
		payload := `{"iss":"issuer","sub":"subject","aud":"audience","iat":1735686000}`
		_, privateKey := genTestKeys(t)

		token, err := GenerateSignedToken([]byte(payload), privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		keyID, err := GetKeyIDFromToken([]byte(token))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keyID).To(Equal(privateKey.KeyID))
	})

	t.Run("fails with invalid token", func(t *testing.T) {
		g := NewWithT(t)

		keyID, err := GetKeyIDFromToken([]byte("invalid-token"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(keyID).To(BeEmpty())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with empty token", func(t *testing.T) {
		g := NewWithT(t)

		keyID, err := GetKeyIDFromToken([]byte(""))
		g.Expect(err).To(HaveOccurred())
		g.Expect(keyID).To(BeEmpty())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with malformed JWT (missing signature)", func(t *testing.T) {
		g := NewWithT(t)
		// JWT with only header and payload, no signature
		malformedToken := "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJ0ZXN0In0"

		keyID, err := GetKeyIDFromToken([]byte(malformedToken))
		g.Expect(err).To(HaveOccurred())
		g.Expect(keyID).To(BeEmpty())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with token missing kid header", func(t *testing.T) {
		g := NewWithT(t)
		_, privateKey := genTestKeys(t)

		// Create a private key without KeyID
		privateKeyNoKID := &EdPrivateKey{
			Key:    privateKey.Key,
			KeyID:  "", // Empty KeyID
			Issuer: privateKey.Issuer,
		}

		payload := `{"iss":"test-issuer"}`
		token, err := GenerateSignedToken([]byte(payload), privateKeyNoKID)
		g.Expect(err).ToNot(HaveOccurred())

		keyID, err := GetKeyIDFromToken([]byte(token))
		g.Expect(err).To(HaveOccurred())
		g.Expect(keyID).To(BeEmpty())
		g.Expect(errors.Is(err, ErrKIDNotFoundInJWT)).To(BeTrue())
	})
}

// genTestKeys generates a test EdPublicKey and EdPrivateKey pair
func genTestKeys(t *testing.T) (*EdPublicKey, *EdPrivateKey) {
	g := NewWithT(t)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	return &EdPublicKey{
			Key:   publicKey,
			KeyID: "test-key-id",
		}, &EdPrivateKey{
			Key:    privateKey,
			KeyID:  "test-key-id",
			Issuer: "test-issuer",
		}
}
