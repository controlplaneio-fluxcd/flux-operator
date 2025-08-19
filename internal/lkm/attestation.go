// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import "encoding/json"

// Attestation represents a cryptographic attestation that contains
// standard claims as defined in RFC 7519 (JSON Web Token)
// and custom claims specific to the Flux distribution.
// RFC7519: https://datatracker.ietf.org/doc/rfc7519
type Attestation struct {
	// ID is the unique identifier UUID v6 for the attestation
	// (RFC 7519 JTI claim).
	// +required
	ID string `json:"jti"`

	// Issuer is the identifier of the entity that issued the attestation
	// (RFC 7519 ISS claim).
	// +required
	Issuer string `json:"iss"`

	// Subject is the identifier of the entity that the attestation is issued for
	// (RFC 7519 SUB claim).
	// +required
	Subject string `json:"sub"`

	// Audience is the intended audience for the attestation
	// (RFC 7519 AUD claim).
	// +required
	Audience string `json:"aud"`

	// Expiry is the expiration time of the attestation in Unix timestamp format
	// (RFC 7519 EXP claim).
	// +optional
	Expiry int64 `json:"exp,omitempty"`

	// IssuedAt is the time when the attestation was issued in Unix timestamp format
	// (RFC 7519 IAT claim).
	// +required
	IssuedAt int64 `json:"iat"`

	// Checksum is the hash used to verify the integrity of the subject's data.
	// +required
	Checksum string `json:"checksum"`
}

// String returns a JSON representation of the Attestation object.
func (a Attestation) String() string {
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "invalid attestation"
	}
	return string(data)
}
