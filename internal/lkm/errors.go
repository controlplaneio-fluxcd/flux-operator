// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"errors"
	"fmt"
)

// ErrClaimAudienceEmpty is returned when the license key audience is empty.
var ErrClaimAudienceEmpty = errors.New("audience (aud) cannot be empty")

// ErrClaimChecksumEmpty is returned when the manifests attestation checksum is empty.
var ErrClaimChecksumEmpty = errors.New("checksum cannot be empty")

// ErrClaimChecksumImmutable is returned when the manifests attestation already contains a checksum.
var ErrClaimChecksumImmutable = errors.New("checksum claim cannot be overwritten")

// ErrClaimChecksumMismatch is returned when the manifests attestation checksum does not match the expected value.
var ErrClaimChecksumMismatch = errors.New("checksum mismatch")

// ErrClaimDigestsEmpty is returned when the attestation digests list is empty.
var ErrClaimDigestsEmpty = errors.New("digests list cannot be empty")

// ErrClaimDigestsImmutable is returned when the attestation digests already contain values.
var ErrClaimDigestsImmutable = errors.New("digests claim cannot be overwritten")

// ErrClaimExpired is returned when the claim expiry time is in the past.
var ErrClaimExpired = errors.New("expiry (exp) cannot be in the past")

// ErrClaimExpiryZero is returned when the license key expiry time is zero.
var ErrClaimExpiryZero = errors.New("expiry (exp) cannot be zero")

// ErrClaimIDEmpty is returned when the license key ID is empty.
var ErrClaimIDEmpty = errors.New("id (jti) cannot be empty")

// ErrClaimIssuedAtFuture is returned when the license key issued at time is in the future.
var ErrClaimIssuedAtFuture = errors.New("issued at (iat) cannot be in the future")

// ErrClaimIssuedAtZero is returned when the license key issued at time is zero.
var ErrClaimIssuedAtZero = errors.New("issued at (iat) cannot be zero")

// ErrClaimIssuerEmpty is returned when the license key issuer is empty.
var ErrClaimIssuerEmpty = errors.New("issuer (iss) cannot be empty")

// ErrClaimSubjectEmpty is returned when the license key subject is empty.
var ErrClaimSubjectEmpty = errors.New("subject (sub) cannot be empty")

// ErrCreateEncrypter is returned when JWE encrypter creation fails.
var ErrCreateEncrypter = errors.New("failed to create encrypter")

// ErrDecryptPayload is returned when payload decryption fails.
var ErrDecryptPayload = errors.New("failed to decrypt payload")

// ErrEncryptPayload is returned when payload encryption fails.
var ErrEncryptPayload = errors.New("failed to encrypt payload")

// ErrKeyAlgNotECDH is returned when a JWK's algorithm is not ECDH-ES+A128KW.
var ErrKeyAlgNotECDH = errors.New("key algorithm must be 'ECDH-ES+A128KW'")

// ErrKeyAlgNotEdDA is returned when a JWK's algorithm is not EdDSA.
var ErrKeyAlgNotEdDA = errors.New("key algorithm must be 'EdDSA'")

// ErrKeyInvalid is returned when a JWK is not valid.
var ErrKeyInvalid = errors.New("key is not valid")

// ErrKeyNotFound is returned when a JWK is not found in the set.
var ErrKeyNotFound = errors.New("key not found in set")

// ErrKeyNotPrivate is returned when a private key is required but a public key is provided.
var ErrKeyNotPrivate = errors.New("provided key is not a private key")

// ErrKeyNotPublic is returned when a public key is required but a private key is provided.
var ErrKeyNotPublic = errors.New("provided key is not a public key")

// ErrKeySetEmpty is returned when a JWK set is empty.
var ErrKeySetEmpty = errors.New("key set is empty")

// ErrKeyUseNotEnc is returned when a JWK's use is not "enc".
var ErrKeyUseNotEnc = errors.New("key use must be 'enc'")

// ErrKeyUseNotSig is returned when a JWK's use is not "sig".
var ErrKeyUseNotSig = errors.New("key use must be 'sig'")

// ErrKIDMissing is returned when a JWK does not contain a key ID.
var ErrKIDMissing = errors.New("key must have an ID")

// ErrKIDNotFoundInJWKs is returned when the given public key ID is not found in the JWKs.
var ErrKIDNotFoundInJWKs = errors.New("public key ID (kid) not found in JWTs")

// ErrKIDNotFoundInHeaders is returned when no key ID is found in the token headers.
var ErrKIDNotFoundInHeaders = errors.New("key ID not found in headers")

// ErrParseClaims is returned when claims cannot be extracted from the token.
var ErrParseClaims = errors.New("failed to parse claims")

// ErrParseJWE is returned when JWE token parsing fails.
var ErrParseJWE = errors.New("failed to parse JWE")

// ErrParseToken is returned when a signed token cannot be parsed.
var ErrParseToken = errors.New("failed to parse JWT token")

// ErrPayloadEmpty is returned when the payload to encrypt is nil.
var ErrPayloadEmpty = errors.New("payload cannot be empty")

// ErrPrivateKeyRequired is returned when a private key is required but not provided.
var ErrPrivateKeyRequired = errors.New("private key is required")

// ErrPublicKeyRequired is returned when a public key is required but not provided.
var ErrPublicKeyRequired = errors.New("public key is required")

// ErrSigNotFound is returned when no signatures are found in the token.
var ErrSigNotFound = errors.New("no signatures found in JWT token")

// ErrInvalidUUID is returned when a string is not a valid UUID v6.
var ErrInvalidUUID = errors.New("invalid UUID v6")

// ErrVerifySig is returned when signature verification fails.
var ErrVerifySig = errors.New("failed to verify signature")

// InvalidLicenseKeyError wraps an error with the "invalid license key" prefix.
func InvalidLicenseKeyError(err error) error {
	return fmt.Errorf("invalid license key: %w", err)
}

// InvalidAttestationError wraps an error with the "invalid attestation" prefix.
func InvalidAttestationError(err error) error {
	return fmt.Errorf("invalid attestation: %w", err)
}
