// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

// LicenseKey represents a license key object that contains
// standard claims as defined in RFC 7519 (JSON Web Token)
// and custom claims specific to the Flux distribution.
// RFC7519: https://datatracker.ietf.org/doc/rfc7519
type LicenseKey struct {
	// ID is the unique identifier for the license key
	// (RFC 7519 JTI claim).
	// +required
	ID string `json:"jti"`

	// Issuer is the identifier of the entity that issued the license key
	// (RFC 7519 ISS claim).
	// +required
	Issuer string `json:"iss"`

	// Subject is the identifier of the entity that the license key is issued to
	// (RFC 7519 SUB claim).
	// +required
	Subject string `json:"sub"`

	// Audience is the intended audience for the license key
	// (RFC 7519 AUD claim).
	// +required
	Audience string `json:"aud"`

	// Expiry is the expiration time of the license key in Unix timestamp format
	// (RFC 7519 EXP claim).
	// +required
	Expiry int64 `json:"exp"`

	// IssuedAt is the time when the license key was issued in Unix timestamp format
	// (RFC 7519 IAT claim).
	// +required
	IssuedAt int64 `json:"iat"`

	// Capabilities is a list of features granted by the license key.
	// +optional
	Capabilities []string `json:"caps,omitempty"`
}
