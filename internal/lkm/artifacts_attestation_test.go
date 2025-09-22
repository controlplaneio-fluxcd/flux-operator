// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

// testArtifactsAttestation returns a valid test ArtifactsAttestation
func testArtifactsAttestation() ArtifactsAttestation {
	now := time.Now()
	return ArtifactsAttestation{
		att: Attestation{
			ID:       "01f080cb-8881-6194-a0de-c69c5184ad4d",
			Issuer:   "test-issuer",
			Subject:  "artifacts",
			Audience: []string{"test-audience"},
			IssuedAt: now.Unix(),
			Digests:  []string{"sha256:abc123", "sha256:def456"},
		},
	}
}

func TestNewArtifactsAttestation(t *testing.T) {
	jti, _ := uuid.NewV6()
	t.Run("creates attestation from valid verified data", func(t *testing.T) {
		g := NewWithT(t)

		// Create valid attestation data
		testAtt := testArtifactsAttestation()
		verifiedData, err := testAtt.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		aa, err := NewArtifactsAttestation(verifiedData)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(aa).ToNot(BeNil())

		att := aa.GetAttestation()
		g.Expect(att.ID).ToNot(BeEmpty())
		g.Expect(att.Issuer).To(Equal("test-issuer"))
		g.Expect(att.Subject).To(Equal("artifacts"))
		g.Expect(att.Audience).To(Equal([]string{"test-audience"}))
		g.Expect(att.Digests).To(Equal([]string{"sha256:abc123", "sha256:def456"}))
		g.Expect(att.IssuedAt).To(BeNumerically(">", 0))
	})

	t.Run("fails with invalid JSON data", func(t *testing.T) {
		g := NewWithT(t)
		// Valid JSON but will fail validation due to missing required fields
		invalidData := []byte(`{"invalid": "json", "missing": "fields"}`)

		aa, err := NewArtifactsAttestation(invalidData)

		g.Expect(err).To(HaveOccurred())
		g.Expect(aa).To(BeNil())
		// This will fail validation, not JSON parsing
		g.Expect(errors.Is(err, ErrClaimIDEmpty)).To(BeTrue())
	})

	t.Run("fails with malformed JSON", func(t *testing.T) {
		g := NewWithT(t)
		malformedData := []byte(`{"id": "test", "issuer":`)

		aa, err := NewArtifactsAttestation(malformedData)

		g.Expect(err).To(HaveOccurred())
		g.Expect(aa).To(BeNil())
		g.Expect(errors.Is(err, ErrParseClaims)).To(BeTrue())
	})

	t.Run("fails validation with missing required fields", func(t *testing.T) {
		g := NewWithT(t)

		// Create attestation with missing ID
		invalidAtt := Attestation{
			Issuer:   "test-issuer",
			Subject:  "artifacts",
			Audience: []string{"test-audience"},
			IssuedAt: time.Now().Unix(),
			Digests:  []string{"sha256:abc123"},
		}
		verifiedData, err := json.Marshal(invalidAtt)
		g.Expect(err).ToNot(HaveOccurred())

		aa, err := NewArtifactsAttestation(verifiedData)

		g.Expect(err).To(HaveOccurred())
		g.Expect(aa).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimIDEmpty)).To(BeTrue())
	})

	t.Run("fails validation with wrong subject", func(t *testing.T) {
		g := NewWithT(t)

		// Create attestation with wrong subject
		invalidAtt := Attestation{
			ID:       jti.String(),
			Issuer:   "test-issuer",
			Subject:  "wrong-subject",
			Audience: []string{"test-audience"},
			IssuedAt: time.Now().Unix(),
			Digests:  []string{"sha256:abc123"},
		}
		verifiedData, err := json.Marshal(invalidAtt)
		g.Expect(err).ToNot(HaveOccurred())

		aa, err := NewArtifactsAttestation(verifiedData)

		g.Expect(err).To(HaveOccurred())
		g.Expect(aa).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("subject must be 'artifacts'"))
	})

	t.Run("fails validation with empty digests", func(t *testing.T) {
		g := NewWithT(t)

		// Create attestation with empty digests
		invalidAtt := Attestation{
			ID:       jti.String(),
			Issuer:   "test-issuer",
			Subject:  "artifacts",
			Audience: []string{"test-audience"},
			IssuedAt: time.Now().Unix(),
			Digests:  []string{},
		}
		verifiedData, err := json.Marshal(invalidAtt)
		g.Expect(err).ToNot(HaveOccurred())

		aa, err := NewArtifactsAttestation(verifiedData)

		g.Expect(err).To(HaveOccurred())
		g.Expect(aa).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimDigestsEmpty)).To(BeTrue())
	})
}

func TestNewArtifactsAttestationForAudience(t *testing.T) {
	t.Run("creates attestation with valid audience", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")

		g.Expect(aa).ToNot(BeNil())
		att := aa.GetAttestation()
		g.Expect(att.Audience).To(Equal([]string{"test-audience"}))
		g.Expect(att.Subject).To(Equal("artifacts"))
		// Other fields should be empty initially
		g.Expect(att.ID).To(BeEmpty())
		g.Expect(att.Issuer).To(BeEmpty())
		g.Expect(att.IssuedAt).To(BeZero())
		g.Expect(att.Digests).To(BeEmpty())
	})

	t.Run("creates attestation with empty audience", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("")

		g.Expect(aa).ToNot(BeNil())
		att := aa.GetAttestation()
		g.Expect(att.Audience).To(BeEmpty())
		g.Expect(att.Subject).To(Equal("artifacts"))
	})
}

func TestArtifactsAttestation_GetAttestation(t *testing.T) {
	g := NewWithT(t)
	testAtt := testArtifactsAttestation()
	aa := &testAtt

	att := aa.GetAttestation()
	g.Expect(att).To(Equal(testAtt.att))
}

func TestArtifactsAttestation_GetIssuer(t *testing.T) {
	t.Run("returns issuer from populated attestation", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt

		issuer := aa.GetIssuer()
		g.Expect(issuer).To(Equal(testAtt.att.Issuer))
		g.Expect(issuer).To(Equal("test-issuer"))
	})

	t.Run("returns empty string for uninitialized attestation", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")

		issuer := aa.GetIssuer()
		g.Expect(issuer).To(BeEmpty())
	})
}

func TestArtifactsAttestation_GetIssuedAt(t *testing.T) {
	t.Run("returns formatted timestamp from populated attestation", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt

		issuedAt := aa.GetIssuedAt()
		expectedTime := time.Unix(testAtt.att.IssuedAt, 0).Format(time.RFC3339)
		g.Expect(issuedAt).To(Equal(expectedTime))
		g.Expect(issuedAt).ToNot(BeEmpty())
	})

	t.Run("returns Unix epoch for uninitialized attestation", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")

		issuedAt := aa.GetIssuedAt()
		expectedTime := time.Unix(0, 0).Format(time.RFC3339)
		g.Expect(issuedAt).To(Equal(expectedTime))
	})
}

func TestArtifactsAttestation_HasDigest(t *testing.T) {
	t.Run("returns true for existing digest", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt

		g.Expect(aa.HasDigest("sha256:abc123")).To(BeTrue())
		g.Expect(aa.HasDigest("sha256:def456")).To(BeTrue())
	})

	t.Run("returns false for non-existing digest", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt

		g.Expect(aa.HasDigest("sha256:nonexistent")).To(BeFalse())
		g.Expect(aa.HasDigest("")).To(BeFalse())
	})

	t.Run("returns false for empty digests", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")

		g.Expect(aa.HasDigest("sha256:abc123")).To(BeFalse())
	})
}

func TestArtifactsAttestation_ToJSON(t *testing.T) {
	t.Run("serializes attestation to JSON", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt

		data, err := aa.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())

		// Verify we can unmarshal it back
		var att Attestation
		err = json.Unmarshal(data, &att)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(att).To(Equal(testAtt.att))
	})

	t.Run("handles empty attestation", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")

		data, err := aa.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())
	})
}

func TestArtifactsAttestation_Sign(t *testing.T) {
	t.Run("successfully signs with digests", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")
		publicKey, privateKey := genTestKeys(t)
		digests := []string{"sha256:abc123", "sha256:def456"}

		token, err := aa.Sign(privateKey, digests)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(strings.Count(token, ".")).To(Equal(2)) // JWT has 3 parts separated by dots

		// Verify attestation was populated
		att := aa.GetAttestation()
		g.Expect(att.ID).ToNot(BeEmpty())
		g.Expect(att.Issuer).To(Equal(privateKey.Issuer))
		g.Expect(att.Subject).To(Equal("artifacts"))
		g.Expect(att.Audience).To(Equal([]string{"test-audience"}))
		g.Expect(att.IssuedAt).To(BeNumerically(">", 0))
		g.Expect(att.Digests).To(Equal(digests))

		// Verify ID is a valid UUID v6
		parsedUUID, err := uuid.Parse(att.ID)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedUUID.Version()).To(Equal(uuid.Version(6)))

		// Verify we can verify the token using VerifySignedToken and create a new attestation
		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(publicKey.Key, publicKey.KeyID)
		g.Expect(err).ToNot(HaveOccurred())
		jwksData, err := keySet.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())

		verifiedPayload, err := VerifySignedToken([]byte(token), jwksData)
		g.Expect(err).ToNot(HaveOccurred())

		aa2, err := NewArtifactsAttestation(verifiedPayload)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the reconstructed attestation matches
		att2 := aa2.GetAttestation()
		g.Expect(att2.ID).To(Equal(att.ID))
		g.Expect(att2.Digests).To(Equal(digests))
	})

	t.Run("successfully signs with single digest", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")
		_, privateKey := genTestKeys(t)
		digests := []string{"sha256:singledigest"}

		token, err := aa.Sign(privateKey, digests)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(aa.GetAttestation().Digests).To(Equal(digests))
	})

	t.Run("fails with nil private key", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")
		digests := []string{"sha256:abc123"}

		token, err := aa.Sign(nil, digests)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(err.Error()).To(ContainSubstring("private key is required"))
	})

	t.Run("fails when attestation already has digests", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testArtifactsAttestation()
		aa := &testAtt
		_, privateKey := genTestKeys(t)
		digests := []string{"sha256:newdigest"}

		token, err := aa.Sign(privateKey, digests)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(errors.Is(err, ErrClaimDigestsImmutable)).To(BeTrue())
	})

	t.Run("fails with empty digests", func(t *testing.T) {
		g := NewWithT(t)
		aa := NewArtifactsAttestationForAudience("test-audience")
		_, privateKey := genTestKeys(t)
		digests := []string{}

		token, err := aa.Sign(privateKey, digests)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(errors.Is(err, ErrClaimDigestsEmpty)).To(BeTrue())
	})
}
