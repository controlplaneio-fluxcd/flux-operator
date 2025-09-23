// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
)

const (
	// UseTypeEnc is the use type for encryption keys.
	UseTypeEnc = "enc"

	// UseTypeSig is the use type for signing keys.
	UseTypeSig = "sig"
)

// NewSigningKeySet generates a new Ed25519 key pair
// and returns a public and private JSON Web Key Set.
// The private key set is associated with the issuer.
// The key ID is generated using UUID v6.
func NewSigningKeySet(issuer string) (publicKeySet *EdKeySet, privateKeySet *EdKeySet, err error) {
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

	return publicKeySet, privateKeySet, nil
}

// NewEncryptionKeySet generates a new ECDSA key pair
// and returns a public and private JSON Web Key Set.
// The key pair is suitable for ECDH-ES+A128KW encryption.
// The key ID is generated using UUID v6.
func NewEncryptionKeySet() (publicKeySet *jose.JSONWebKeySet, privateKeySet *jose.JSONWebKeySet, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	kid, err := uuid.NewV6()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key ID: %w", err)
	}

	publicKeySet = &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       key.Public(),
				KeyID:     kid.String(),
				Algorithm: string(jose.ECDH_ES_A128KW),
				Use:       UseTypeEnc,
			},
		},
	}

	privateKeySet = &jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       key,
				KeyID:     kid.String(),
				Algorithm: string(jose.ECDH_ES_A128KW),
				Use:       UseTypeEnc,
			},
		},
	}

	return publicKeySet, privateKeySet, nil
}

// WriteEncryptionKeySet writes the JWKs containing ECDH-ES+A128KW
// public or private keys to a file.
func WriteEncryptionKeySet(filename string, keySet *jose.JSONWebKeySet) error {
	// Validate the key set
	if keySet == nil || len(keySet.Keys) == 0 {
		return ErrKeySetEmpty
	}
	for _, jwk := range keySet.Keys {
		if err := validateEncryptionKey(jwk); err != nil {
			return err
		}
	}

	// Default permissions for the public key set
	// is readable by everyone, writable by owner (0644).
	perm := os.FileMode(0644)
	if !keySet.Keys[0].IsPublic() {
		// For private key set restrict permissions
		// to the owner only (0600).
		perm = 0600
	}

	// Serialize the key set to JSON
	data, err := json.MarshalIndent(keySet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key set: %w", err)
	}

	// Write the JSON data to the specified file
	if err := os.WriteFile(filename, data, perm); err != nil {
		return fmt.Errorf("failed to write key set to file: %w", err)
	}
	return nil
}

// ReadEncryptionKeySet reads a file containing ECDH-ES+A128KW
// public or private keys and returns a JWKs.
func ReadEncryptionKeySet(filename string) (*jose.JSONWebKeySet, error) {
	// Read the file content
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read key set from file %s: %w", filename, err)
	}

	// Unmarshal the JSON data into a JSONWebKeySet
	var keySet jose.JSONWebKeySet
	if err := json.Unmarshal(data, &keySet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key set: %w", err)
	}

	// Validate the key set
	if len(keySet.Keys) == 0 {
		return nil, ErrKeySetEmpty
	}
	for _, jwk := range keySet.Keys {
		if err := validateEncryptionKey(jwk); err != nil {
			return nil, err
		}
	}

	return &keySet, nil
}

func validateEncryptionKey(jwk jose.JSONWebKey) error {
	if jwk.KeyID == "" {
		return ErrKIDMissing
	}
	if jwk.Use != UseTypeEnc {
		return fmt.Errorf("key %s has unsupported use", jwk.KeyID)
	}
	if jwk.Algorithm != string(jose.ECDH_ES_A128KW) {
		return fmt.Errorf("key %s has unsupported algorithm", jwk.KeyID)
	}
	if !jwk.Valid() {
		return fmt.Errorf("key %s is not valid", jwk.KeyID)
	}
	return nil
}
