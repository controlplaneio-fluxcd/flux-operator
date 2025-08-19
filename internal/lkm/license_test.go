// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

// testLicenseKey returns a valid LicenseKey for testing
func testLicenseKey() LicenseKey {
	now := time.Now()
	return LicenseKey{
		ID:           "test-id",
		Issuer:       "test-issuer",
		Subject:      "test-subject",
		Audience:     "test-audience",
		IssuedAt:     now.Unix(),
		Expiry:       now.Add(24 * time.Hour).Unix(),
		Capabilities: []string{"feature1", "feature2"},
	}
}

// testEdPrivateKey generates a test EdPrivateKey and EdPublicKey pair
func testEdPrivateKey(t *testing.T) (*EdPrivateKey, *EdPublicKey) {
	g := NewWithT(t)

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	return &EdPrivateKey{
			Key:    privateKey,
			KeyID:  "test-key-id",
			Issuer: "test-issuer",
		}, &EdPublicKey{
			Key:   publicKey,
			KeyID: "test-key-id",
		}
}

func TestNewLicense(t *testing.T) {

	t.Run("creates license with valid parameters", func(t *testing.T) {
		g := NewWithT(t)

		issuer := "test-issuer"
		subject := "Test Company LLC"
		audience := "flux-operator"
		expiryInHours := 24
		capabilities := []string{"feature1", "feature2"}

		license, err := NewLicense(issuer, subject, audience, time.Duration(expiryInHours)*time.Hour, capabilities)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(license).ToNot(BeNil())

		// Verify the license key fields
		lk := license.GetKey()
		g.Expect(lk.Issuer).To(Equal(issuer))
		g.Expect(lk.Audience).To(Equal(audience))
		g.Expect(lk.Capabilities).To(Equal(capabilities))

		// Verify subject is anonymized with "c-" prefix and 8-character hex
		g.Expect(lk.Subject).To(HavePrefix("c-"))
		g.Expect(lk.Subject).To(HaveLen(10)) // "c-" + 8 hex characters

		// Verify ID is a valid UUID v6 format
		parsedUUID, err := uuid.Parse(lk.ID)
		g.Expect(err).ToNot(HaveOccurred(), "ID should be a valid UUID")
		g.Expect(parsedUUID.Version()).To(Equal(uuid.Version(6)), "should be UUID v6")

		// Verify timestamps are reasonable (within last minute and future)
		now := time.Now().Unix()
		g.Expect(lk.IssuedAt).To(BeNumerically("~", now, 60))
		expectedExpiry := now + int64(expiryInHours*3600)
		g.Expect(lk.Expiry).To(BeNumerically("~", expectedExpiry, 60))
	})

	t.Run("creates license with empty capabilities", func(t *testing.T) {
		g := NewWithT(t)

		license, err := NewLicense("issuer", "subject", "audience", 1, nil)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(license).ToNot(BeNil())
		g.Expect(license.GetKey().Capabilities).To(BeNil())
	})

	t.Run("creates license with zero expiry hours", func(t *testing.T) {
		g := NewWithT(t)

		license, err := NewLicense("issuer", "subject", "audience", 0, []string{})

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(license).ToNot(BeNil())

		lk := license.GetKey()
		// With 0 hours, expiry should be approximately equal to issued at
		g.Expect(lk.Expiry).To(BeNumerically("~", lk.IssuedAt, 2))
	})

	t.Run("creates license with negative expiry hours", func(t *testing.T) {
		g := NewWithT(t)

		license, err := NewLicense("issuer", "subject", "audience", -1*time.Hour, []string{})

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(license).ToNot(BeNil())

		lk := license.GetKey()
		// With -1 hours, expiry should be 1 hour before issued at
		g.Expect(lk.Expiry).To(BeNumerically("~", lk.IssuedAt-3600, 2))
	})

	t.Run("anonymizes different subjects consistently", func(t *testing.T) {
		g := NewWithT(t)

		// Same subject should produce same anonymized ID
		license1, err := NewLicense("issuer", "Test Company", "audience", 1, nil)
		g.Expect(err).ToNot(HaveOccurred())

		license2, err := NewLicense("issuer", "Test Company", "audience", 1, nil)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(license1.GetKey().Subject).To(Equal(license2.GetKey().Subject))

		// Different subject should produce different anonymized ID
		license3, err := NewLicense("issuer", "Different Company", "audience", 1, nil)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(license1.GetKey().Subject).ToNot(Equal(license3.GetKey().Subject))
	})

	t.Run("fails with empty issuer", func(t *testing.T) {
		g := NewWithT(t)

		license, err := NewLicense("", "subject", "audience", 1, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(license).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimIssuerEmpty)).To(BeTrue())
	})

	t.Run("fails with empty audience", func(t *testing.T) {
		g := NewWithT(t)

		license, err := NewLicense("issuer", "subject", "", 1, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(license).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimAudienceEmpty)).To(BeTrue())
	})
}

func TestNewLicenseWithKey(t *testing.T) {

	t.Run("creates license with valid key", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()

		license, err := NewLicenseWithKey(lk)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(license).ToNot(BeNil())
		g.Expect(license.lk).To(Equal(lk))
	})

	t.Run("fails with invalid key", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.ID = "" // Make it invalid

		license, err := NewLicenseWithKey(lk)

		g.Expect(err).To(HaveOccurred())
		g.Expect(license).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimIDEmpty)).To(BeTrue())
	})
}

func TestLicense_GetMethods(t *testing.T) {
	g := NewWithT(t)
	lk := testLicenseKey()
	license, err := NewLicenseWithKey(lk)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("GetKey returns license key", func(t *testing.T) {
		g := NewWithT(t)
		key := license.GetKey()
		g.Expect(key).To(Equal(lk))
	})

	t.Run("GetIssuer returns issuer", func(t *testing.T) {
		g := NewWithT(t)
		issuer := license.GetIssuer()
		g.Expect(issuer).To(Equal(lk.Issuer))
	})

	t.Run("GetExpiry returns formatted expiry", func(t *testing.T) {
		g := NewWithT(t)
		expiry := license.GetExpiry()
		expected := time.Unix(lk.Expiry, 0).Format(time.RFC3339)
		g.Expect(expiry).To(Equal(expected))
	})

	t.Run("GetIssuedAt returns formatted issued at", func(t *testing.T) {
		g := NewWithT(t)
		issuedAt := license.GetIssuedAt()
		expected := time.Unix(lk.IssuedAt, 0).Format(time.RFC3339)
		g.Expect(issuedAt).To(Equal(expected))
	})
}

func TestLicense_Validate(t *testing.T) {

	t.Run("valid license passes validation", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("fails when ID is empty", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.ID = ""
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIDEmpty)).To(BeTrue())
	})

	t.Run("fails when Issuer is empty", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Issuer = ""
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuerEmpty)).To(BeTrue())
	})

	t.Run("fails when Subject is empty", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Subject = ""
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimSubjectEmpty)).To(BeTrue())
	})

	t.Run("fails when Audience is empty", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Audience = ""
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimAudienceEmpty)).To(BeTrue())
	})

	t.Run("fails when IssuedAt is zero", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.IssuedAt = 0
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuedAtZero)).To(BeTrue())
	})

	t.Run("fails when IssuedAt is in the future", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.IssuedAt = time.Now().Add(2 * time.Minute).Unix()
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuedAtFuture)).To(BeTrue())
	})

	t.Run("fails when Expiry is zero", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Expiry = 0
		license := &License{lk: lk}

		err := license.Validate()
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimExpiryZero)).To(BeTrue())
	})
}

func TestLicense_IsExpired(t *testing.T) {

	t.Run("returns false for valid license", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		expired := license.IsExpired(time.Minute)
		g.Expect(expired).To(BeFalse())
	})

	t.Run("returns true for expired license", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Expiry = time.Now().Add(-time.Hour).Unix() // Expired 1 hour ago
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		expired := license.IsExpired(time.Minute)
		g.Expect(expired).To(BeTrue())
	})

	t.Run("respects leeway for clock skew", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Expiry = time.Now().Add(-30 * time.Second).Unix() // Expired 30s ago
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		// With 1 minute leeway, should not be considered expired
		expired := license.IsExpired(time.Minute)
		g.Expect(expired).To(BeFalse())

		// With 10 second leeway, should be considered expired
		expired = license.IsExpired(10 * time.Second)
		g.Expect(expired).To(BeTrue())
	})
}

func TestLicense_HasAudience(t *testing.T) {
	g := NewWithT(t)
	lk := testLicenseKey()
	lk.Audience = "test-audience"
	license, err := NewLicenseWithKey(lk)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("returns true for exact match", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasAudience("test-audience")
		g.Expect(has).To(BeTrue())
	})

	t.Run("returns true for case insensitive match", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasAudience("TEST-AUDIENCE")
		g.Expect(has).To(BeTrue())
	})

	t.Run("returns false for non-match", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasAudience("other-audience")
		g.Expect(has).To(BeFalse())
	})
}

func TestLicense_HasCapability(t *testing.T) {
	g := NewWithT(t)
	lk := testLicenseKey()
	lk.Capabilities = []string{"feature1", "feature2"}
	license, err := NewLicenseWithKey(lk)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("returns true for existing capability", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasCapability("feature1")
		g.Expect(has).To(BeTrue())
	})

	t.Run("returns true for case insensitive match", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasCapability("FEATURE1")
		g.Expect(has).To(BeTrue())
	})

	t.Run("returns false for non-existing capability", func(t *testing.T) {
		g := NewWithT(t)
		has := license.HasCapability("feature3")
		g.Expect(has).To(BeFalse())
	})

	t.Run("returns false for empty capabilities", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		lk.Capabilities = nil
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		has := license.HasCapability("feature1")
		g.Expect(has).To(BeFalse())
	})
}

func TestLicense_ToJSON(t *testing.T) {
	g := NewWithT(t)
	lk := testLicenseKey()
	license, err := NewLicenseWithKey(lk)
	g.Expect(err).ToNot(HaveOccurred())

	data, err := license.ToJSON()

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(data).ToNot(BeEmpty())

	// Verify it can be unmarshaled back
	var parsed LicenseKey
	err = json.Unmarshal(data, &parsed)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(parsed).To(Equal(lk))
}

func TestLicense_Sign(t *testing.T) {

	t.Run("successfully signs license", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		privateKey, _ := testEdPrivateKey(t)

		token, err := license.Sign(privateKey)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(strings.Count(token, ".")).To(Equal(2)) // JWT has 3 parts separated by dots
	})

	t.Run("fails with nil private key", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		token, err := license.Sign(nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(errors.Is(err, ErrPrivateKeyRequired)).To(BeTrue())
	})
}

func TestGetKeyIDFromToken(t *testing.T) {

	t.Run("extracts key ID from valid token", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		privateKey, _ := testEdPrivateKey(t)
		token, err := license.Sign(privateKey)
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
}

func TestGetLicenseFromToken(t *testing.T) {

	t.Run("successfully extracts license from token", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		originalLicense, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		privateKey, publicKey := testEdPrivateKey(t)
		token, err := originalLicense.Sign(privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		extractedLicense, err := GetLicenseFromToken([]byte(token), publicKey)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(extractedLicense).ToNot(BeNil())
		g.Expect(extractedLicense.GetKey()).To(Equal(lk))
	})

	t.Run("fails with nil public key", func(t *testing.T) {
		g := NewWithT(t)
		license, err := GetLicenseFromToken([]byte("token"), nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(license).To(BeNil())
		g.Expect(errors.Is(err, ErrPublicKeyRequired)).To(BeTrue())
	})

	t.Run("fails with invalid token", func(t *testing.T) {
		g := NewWithT(t)
		_, publicKey := testEdPrivateKey(t)

		license, err := GetLicenseFromToken([]byte("invalid-token"), publicKey)

		g.Expect(err).To(HaveOccurred())
		g.Expect(license).To(BeNil())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with wrong public key", func(t *testing.T) {
		g := NewWithT(t)
		lk := testLicenseKey()
		license, err := NewLicenseWithKey(lk)
		g.Expect(err).ToNot(HaveOccurred())

		privateKey, _ := testEdPrivateKey(t)
		token, err := license.Sign(privateKey)
		g.Expect(err).ToNot(HaveOccurred())

		// Use different public key
		_, wrongPublicKey := testEdPrivateKey(t)

		extractedLicense, err := GetLicenseFromToken([]byte(token), wrongPublicKey)

		g.Expect(err).To(HaveOccurred())
		g.Expect(extractedLicense).To(BeNil())
		g.Expect(errors.Is(err, ErrVerifySig)).To(BeTrue())
	})
}
