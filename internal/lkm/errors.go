// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"errors"
	"fmt"
)

// ErrParseToken is returned when a signed token cannot be parsed.
var ErrParseToken = errors.New("failed to parse signed token")

// ErrSigNotFound is returned when no signatures are found in the token.
var ErrSigNotFound = errors.New("no signatures found")

// ErrKIDNotFound is returned when no public key ID is found in the token.
var ErrKIDNotFound = errors.New("no public key ID found")

// ErrPublicKeyRequired is returned when a public key is required but not provided.
var ErrPublicKeyRequired = errors.New("public key is required")

// ErrPrivateKeyRequired is returned when a private key is required but not provided.
var ErrPrivateKeyRequired = errors.New("private key is required")

// ErrVerifySig is returned when signature verification fails.
var ErrVerifySig = errors.New("failed to verify signature")

// ErrParseClaims is returned when claims cannot be extracted from the token.
var ErrParseClaims = errors.New("failed to parse claims")

// ErrClaimIDEmpty is returned when the license key ID is empty.
var ErrClaimIDEmpty = errors.New("id (jti) cannot be empty")

// ErrClaimIssuerEmpty is returned when the license key issuer is empty.
var ErrClaimIssuerEmpty = errors.New("issuer (iss) cannot be empty")

// ErrClaimSubjectEmpty is returned when the license key subject is empty.
var ErrClaimSubjectEmpty = errors.New("subject (sub) cannot be empty")

// ErrClaimAudienceEmpty is returned when the license key audience is empty.
var ErrClaimAudienceEmpty = errors.New("audience (aud) cannot be empty")

// ErrClaimIssuedAtZero is returned when the license key issued at time is zero.
var ErrClaimIssuedAtZero = errors.New("issued at (iat) cannot be zero")

// ErrClaimIssuedAtFuture is returned when the license key issued at time is in the future.
var ErrClaimIssuedAtFuture = errors.New("issued at (iat) cannot be in the future")

// ErrClaimExpiryZero is returned when the license key expiry time is zero.
var ErrClaimExpiryZero = errors.New("expiry (exp) cannot be zero")

// ErrClaimChecksumEmpty is returned when the attestation checksum is empty.
var ErrClaimChecksumEmpty = errors.New("checksum cannot be empty")

// ErrClaimChecksumMismatch is returned when the attestation checksum does not match the expected value.
var ErrClaimChecksumMismatch = errors.New("checksum mismatch")

// InvalidLicenseKeyError wraps an error with the "invalid license key" prefix.
func InvalidLicenseKeyError(err error) error {
	return fmt.Errorf("invalid license key: %w", err)
}

// InvalidAttestationError wraps an error with the "invalid attestation" prefix.
func InvalidAttestationError(err error) error {
	return fmt.Errorf("invalid attestation: %w", err)
}
