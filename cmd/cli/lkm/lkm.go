// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Package lkm provides license key verification.
package lkm

import (
	"fmt"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/lkm"
)

// VerifyResult contains the verified license key information.
type VerifyResult struct {
	ID           string
	Issuer       string
	IssuedAt     string
	Expiry       string
	Capabilities []string
}

// VerifyLicenseKey verifies a signed JWT license key using the provided
// JWKS public key set. It validates the signature, claims, expiry,
// checks that the license is not revoked (if revokedKeysJSON is non-nil),
// and checks that the license contains all required SKU capabilities.
func VerifyLicenseKey(jwksData, jwtData, revokedKeysJSON []byte, skus ...string) (*VerifyResult, error) {
	kid, err := lkm.GetKeyIDFromToken(jwtData)
	if err != nil {
		return nil, lkm.InvalidLicenseKeyError(err)
	}

	pk, err := lkm.EdPublicKeyFromSet(jwksData, kid)
	if err != nil {
		return nil, lkm.InvalidLicenseKeyError(err)
	}

	lic, err := lkm.GetLicenseFromToken(jwtData, pk)
	if err != nil {
		return nil, err
	}

	if len(revokedKeysJSON) > 0 {
		rks, err := lkm.RevocationKeySetFromJSON(revokedKeysJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to parse revoked key set: %w", err)
		}
		if revoked, ts := rks.IsRevoked(lic); revoked {
			return nil, fmt.Errorf("license key is revoked since %s", ts)
		}
	}

	if lic.IsExpired(time.Second) {
		return nil, fmt.Errorf("license key has expired on %s", lic.GetExpiry())
	}

	for _, s := range skus {
		if !lic.HasCapability(s) {
			return nil, fmt.Errorf("sku not found in license: required capability %q is missing", s)
		}
	}

	return &VerifyResult{
		ID:           lic.GetKey().ID,
		Issuer:       lic.GetIssuer(),
		IssuedAt:     lic.GetIssuedAt(),
		Expiry:       lic.GetExpiry(),
		Capabilities: lic.GetKey().Capabilities,
	}, nil
}
