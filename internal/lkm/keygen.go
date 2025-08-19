// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
)

// NewKeySetPair generates a new Ed25519 key pair and returns
// a public EdKeySet and a private EdKeySet with the given issuer.
// The key ID is generated using a UUID v4.
func NewKeySetPair(issuer string) (publicKeySet *EdKeySet, privateKeySet *EdKeySet, err error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	kid, err := uuid.NewV6()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key ID: %w", err)
	}

	publicKeySet = NewPublicKeySet()
	err = publicKeySet.AddPublicKey(publicKey, kid.String())
	if err != nil {
		return nil, nil, err
	}

	privateKeySet = NewPrivateKeySet(issuer)
	err = privateKeySet.AddPrivateKey(privateKey, kid.String())
	if err != nil {
		return nil, nil, err
	}
	return
}

// GetKeyIDFromToken extracts the EdPublicKey ID (KID header) from a signed JWT token.
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
		return "", ErrKIDNotFound
	}

	return kid, nil
}
