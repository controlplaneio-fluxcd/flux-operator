// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// testAttestation returns a valid test Attestation
func testAttestation() Attestation {
	return Attestation{
		ID:       "01f080cb-8881-6194-a0de-c69c5184ad4d",
		Issuer:   "test-issuer",
		Subject:  "artifacts",
		Audience: []string{"test-audience"},
		IssuedAt: time.Now().Unix(),
		Digests: []string{
			"sha256:9b2225dcba561daf2e58f004a37704232b1bae7c65af41693aad259e7cce5150",
			"sha256:52c30b7b1b998045cb5b6a70a19430feb88425aca6c84afe0fde69e0cf5302ae",
		},
	}
}

func TestAttestation_Validate(t *testing.T) {
	t.Run("valid attestation passes validation", func(t *testing.T) {
		g := NewWithT(t)

		err := testAttestation().Validate("test-audience", "artifacts")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("fails when ID is empty", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.ID = ""

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIDEmpty)).To(BeTrue())
	})

	t.Run("fails when Issuer is empty", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Issuer = ""

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuerEmpty)).To(BeTrue())
	})

	t.Run("fails when Subject is empty", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Subject = ""

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimSubjectEmpty)).To(BeTrue())
	})

	t.Run("fails when Subject mismatch", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Subject = "manifests"

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("subject must be 'artifacts'"))
	})

	t.Run("fails when Audience is empty", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Audience = []string{}

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimAudienceEmpty)).To(BeTrue())
	})

	t.Run("fails when IssuedAt is zero", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.IssuedAt = 0

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuedAtZero)).To(BeTrue())
	})

	t.Run("fails when IssuedAt is in the future", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.IssuedAt = time.Now().Add(time.Hour).Unix()

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimIssuedAtFuture)).To(BeTrue())
	})

	t.Run("fails when Expiry is in the past", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Expiry = time.Now().Add(-time.Hour).Unix()

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimExpired)).To(BeTrue())
	})

	t.Run("allows Expiry within past tolerance", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Expiry = time.Now().Add(-29 * time.Second).Unix()

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("allows IssuedAt within future tolerance", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.IssuedAt = time.Now().Add(29 * time.Second).Unix()

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("fails when Subject is not 'artifacts'", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Subject = "wrong-subject"

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("subject must be 'artifacts'"))
	})

	t.Run("fails when Digests is empty", func(t *testing.T) {
		g := NewWithT(t)
		testAtt := testAttestation()
		testAtt.Digests = []string{}

		err := testAtt.Validate("", "artifacts")
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, ErrClaimDigestsEmpty)).To(BeTrue())
	})
}
