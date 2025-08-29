// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ArtifactsAttestation represents an attestation for offline verification
// of artifacts authenticity and integrity.
// It provides methods to sign, verify, and validate the claims of the attestation.
type ArtifactsAttestation struct {
	att Attestation
}

// NewArtifactsAttestation creates a new ArtifactsAttestation instance
// from the provided verified data, which should be a JSON representation
// of the Attestation claims extracted with VerifySignedToken.
// It returns an error if the data cannot be parsed or if the claims are invalid.
func NewArtifactsAttestation(verifiedData []byte) (*ArtifactsAttestation, error) {
	var att Attestation
	if err := json.Unmarshal(verifiedData, &att); err != nil {
		return nil, InvalidAttestationError(ErrParseClaims)
	}

	m := &ArtifactsAttestation{
		att: att,
	}

	if err := m.att.Validate("", "artifacts"); err != nil {
		return nil, err
	}

	return m, nil
}

// NewArtifactsAttestationForAudience creates a new ArtifactsAttestation instance for the specified audience.
func NewArtifactsAttestationForAudience(audience string) *ArtifactsAttestation {
	return &ArtifactsAttestation{
		att: Attestation{
			Audience: audience,
			Subject:  "artifacts",
		},
	}
}

// GetAttestation returns the underlying Attestation object.
func (m *ArtifactsAttestation) GetAttestation() Attestation {
	return m.att
}

// GetIssuer returns the issuer of the underlying Attestation.
func (m *ArtifactsAttestation) GetIssuer() string {
	return m.att.Issuer
}

// GetIssuedAt returns the timestamp when the underlying Attestation
// was issued, formatted as an RFC3339 string.
func (m *ArtifactsAttestation) GetIssuedAt() string {
	return time.Unix(m.att.IssuedAt, 0).Format(time.RFC3339)
}

// HasDigest checks if the underlying Attestation contains the specified digest.
func (m *ArtifactsAttestation) HasDigest(digest string) bool {
	for _, d := range m.att.Digests {
		if d == digest {
			return true
		}
	}
	return false
}

// ToJSON serializes the underlying Attestation to JSON format.
func (m *ArtifactsAttestation) ToJSON() ([]byte, error) {
	data, err := json.Marshal(m.att)
	if err != nil {
		return nil, InvalidAttestationError(err)
	}
	return data, nil
}

// Sign generates a signed JWT token for the given digests using the provided private key.
func (m *ArtifactsAttestation) Sign(privateKey *EdPrivateKey, digests []string) (string, error) {
	if privateKey == nil {
		return "", ErrPrivateKeyRequired
	}
	if len(m.att.Digests) != 0 {
		return "", ErrClaimDigestsImmutable
	}

	// Generate the license ID.
	jti, err := uuid.NewV6()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Set the attestation fields.
	m.att.ID = jti.String()
	m.att.Issuer = privateKey.Issuer
	m.att.IssuedAt = time.Now().Unix()
	m.att.Digests = digests

	// Validate the attestation.
	if err := m.att.Validate("", "artifacts"); err != nil {
		return "", err
	}

	// Marshal the claims to JSON
	payload, err := m.ToJSON()
	if err != nil {
		return "", err
	}

	return GenerateSignedToken(payload, privateKey)
}
