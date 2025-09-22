// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
)

// License represents a license object that contains a LicenseKey.
// It provides methods to sign, verify and validate the claims of the LicenseKey.
type License struct {
	lk LicenseKey
}

// NewLicense creates a new License for the given issuer, subject,
// audience, expiry time duration, and capabilities.
// The subject is anonymized and stored in the license key as 'c-<adler32checksum>'.
// It generates a unique and chronologically sortable ID for the license key using UUID v6.
func NewLicense(issuer, subject, audience string, expiry time.Duration, capabilities []string) (*License, error) {
	// Validate input parameters
	if issuer == "" {
		return nil, InvalidLicenseKeyError(ErrClaimIssuerEmpty)
	}
	if subject == "" {
		return nil, InvalidLicenseKeyError(ErrClaimSubjectEmpty)
	}
	if audience == "" {
		return nil, InvalidLicenseKeyError(ErrClaimAudienceEmpty)
	}

	// Generate the license ID.
	jti, err := uuid.NewV6()
	if err != nil {
		return nil, fmt.Errorf("failed to generate UUID for license key: %w", err)
	}

	// Anonymize the subject.
	hash := sha256.Sum256([]byte(subject))
	subjectID := fmt.Sprintf("c-%016x", hash[:8])

	// Calculate the expiry time based on the current time
	// and the provided duration.
	now := time.Now()
	expiryTime := now.Add(expiry)

	// Create the LicenseKey object with the required fields.
	lk := LicenseKey{
		ID:           jti.String(),
		IssuedAt:     now.Unix(),
		Expiry:       expiryTime.Unix(),
		Issuer:       issuer,
		Subject:      subjectID,
		Audience:     []string{audience},
		Capabilities: capabilities,
	}
	return NewLicenseWithKey(lk)
}

// NewLicenseWithKey creates a new License object with the given LicenseKey.
// if the LicenseKey is invalid, it returns an error.
func NewLicenseWithKey(lk LicenseKey) (*License, error) {
	l := &License{
		lk: lk,
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return l, nil
}

// GetKey returns the LicenseKey object.
func (lic *License) GetKey() LicenseKey {
	return lic.lk
}

// GetIssuer returns the issuer of the License.
func (lic *License) GetIssuer() string {
	return lic.lk.Issuer
}

// GetExpiry returns the expiry time of the License in RFC3339 format.
func (lic *License) GetExpiry() string {
	return time.Unix(lic.lk.Expiry, 0).Format(time.RFC3339)
}

// GetIssuedAt returns the issued at time of the License in RFC3339 format.
func (lic *License) GetIssuedAt() string {
	return time.Unix(lic.lk.IssuedAt, 0).Format(time.RFC3339)
}

// Validate checks if the LicenseKey contains all required fields
// and that the timestamps are valid.
func (lic *License) Validate() error {
	if lic.lk.ID == "" {
		return InvalidLicenseKeyError(ErrClaimIDEmpty)
	}
	if err := validateUUID(lic.lk.ID); err != nil {
		return InvalidLicenseKeyError(err)
	}
	if lic.lk.Issuer == "" {
		return InvalidLicenseKeyError(ErrClaimIssuerEmpty)
	}
	if lic.lk.Subject == "" {
		return InvalidLicenseKeyError(ErrClaimSubjectEmpty)
	}
	if len(lic.lk.Audience) == 0 {
		return InvalidLicenseKeyError(ErrClaimAudienceEmpty)
	}

	if lic.lk.IssuedAt <= 0 {
		return InvalidLicenseKeyError(ErrClaimIssuedAtZero)
	}
	if time.Unix(lic.lk.IssuedAt, 0).After(time.Now().Add(30 * time.Second)) {
		return InvalidLicenseKeyError(ErrClaimIssuedAtFuture)
	}

	if lic.lk.Expiry <= 0 {
		return InvalidLicenseKeyError(ErrClaimExpiryZero)
	}
	return nil
}

// IsExpired checks if the license has expired based on the current time.
// It allows for a leeway period to account for clock skew.
func (lic *License) IsExpired(leeway time.Duration) bool {
	expirationTime := time.Unix(lic.lk.Expiry, 0)
	return time.Now().Add(-leeway).After(expirationTime)
}

// HasAudience checks if the license is intended for the specified audience.
func (lic *License) HasAudience(audience string) bool {
	return slices.ContainsFunc(lic.lk.Audience, func(aud string) bool {
		return strings.EqualFold(aud, audience)
	})
}

// HasCapability checks if the license contains the specified capability.
func (lic *License) HasCapability(capability string) bool {
	return slices.ContainsFunc(lic.lk.Capabilities, func(cap string) bool {
		return strings.EqualFold(cap, capability)
	})
}

// ToJSON converts the License to a JSON byte slice.
func (lic *License) ToJSON() ([]byte, error) {
	data, err := json.Marshal(lic.lk)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal license key: %w", err)
	}
	return data, nil
}

// Sign returns a JWT token signed with the provided EdPrivateKey.
func (lic *License) Sign(privateKey *EdPrivateKey) (string, error) {
	if privateKey == nil {
		return "", ErrPrivateKeyRequired
	}

	// Marshal the claims to JSON
	payload, err := lic.ToJSON()
	if err != nil {
		return "", err
	}

	return GenerateSignedToken(payload, privateKey)
}

// GetLicenseFromToken extracts the License from a signed JWT token.
// It returns an error if the token signature cannot be verified using
// the provided EdPublicKey or if the JWT claims of the LicenseKey are invalid.
func GetLicenseFromToken(jwtData []byte, publicKey *EdPublicKey) (*License, error) {
	if publicKey == nil {
		return nil, ErrPublicKeyRequired
	}

	jws, err := jose.ParseSigned(string(jwtData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return nil, InvalidLicenseKeyError(ErrParseToken)
	}

	payload, err := jws.Verify(publicKey.Key)
	if err != nil {
		return nil, InvalidLicenseKeyError(ErrVerifySig)
	}

	var lk LicenseKey
	if err := json.Unmarshal(payload, &lk); err != nil {
		return nil, InvalidLicenseKeyError(ErrParseClaims)
	}

	return NewLicenseWithKey(lk)
}

// validateUUID checks if the provided string is a valid UUID v6.
func validateUUID(id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ErrInvalidUUID
	}

	if parsed.Version() != uuid.Version(6) {
		return ErrInvalidUUID
	}

	return nil
}
