// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"fmt"

	"github.com/go-jose/go-jose/v4"
)

// GenerateSignedToken signs the JSON-encoded byte slice using the EdPrivateKey and returns a JWT token.
// The returned token contains the KID header set to the private key's KeyID.
func GenerateSignedToken(payload []byte, privateKey *EdPrivateKey) (string, error) {
	if privateKey == nil {
		return "", ErrPrivateKeyRequired
	}

	// Create the signer options
	signerOpts := jose.SignerOptions{}
	signerOpts.WithType("JWT")

	// Set the KID header to the private key's KeyID
	signerOpts.WithHeader("kid", privateKey.KeyID)

	// Create the signer using the Ed25519 private key
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.EdDSA,
		Key:       privateKey.Key,
	}, &signerOpts)
	if err != nil {
		return "", fmt.Errorf("failed to create signer: %w", err)
	}

	// Sign the payload
	signedObject, err := signer.Sign(payload)
	if err != nil {
		return "", fmt.Errorf("failed to sign payload: %w", err)
	}

	// Serialize the signed object to a compact JWT string
	jwt, err := signedObject.CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize signed token: %w", err)
	}

	return jwt, nil
}

// VerifySignedToken extracts the public key from the JWKs data using the KID
// from the JWT token, and verifies the JWT signature using the public key.
// It returns the verified payload containing the JWT claims if successful,
// or an error if verification fails.
func VerifySignedToken(jwtData []byte, jwksData []byte) ([]byte, error) {
	// Parse the signed JWT token
	jws, err := jose.ParseSigned(string(jwtData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return nil, ErrParseToken
	}
	if len(jws.Signatures) == 0 {
		return nil, ErrSigNotFound
	}

	// Extract the KID from the JWT headers
	kid := jws.Signatures[0].Protected.KeyID
	if kid == "" {
		return nil, ErrKIDNotFoundInJWT
	}

	// Read the public key for the specific key ID
	publicKey, err := EdPublicKeyFromSet(jwksData, kid)
	if err != nil {
		return nil, ErrKIDNotFoundInJWKs
	}

	// Verify the JWT signature using the public key
	verifiedPayload, err := jws.Verify(publicKey.Key)
	if err != nil {
		return nil, ErrVerifySig
	}

	return verifiedPayload, nil
}

// GetKeyIDFromToken extracts the KID header from a signed JWT token.
func GetKeyIDFromToken(jwtData []byte) (string, error) {
	jws, err := jose.ParseSigned(string(jwtData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return "", ErrParseToken
	}

	if len(jws.Signatures) == 0 {
		return "", ErrSigNotFound
	}

	kid := jws.Signatures[0].Protected.KeyID
	if kid == "" {
		return "", ErrKIDNotFoundInJWT
	}

	return kid, nil
}
