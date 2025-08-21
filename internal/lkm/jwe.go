// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"fmt"

	"github.com/go-jose/go-jose/v4"
)

// EncryptTokenWithKeySet encrypts a payload using ECDH-ES+A128KW with the provided public key set.
// If kid is empty, uses the first public key in the set.
// Returns a JWE compact serialization string.
func EncryptTokenWithKeySet(payload []byte, keySet *jose.JSONWebKeySet, kid string) (string, error) {
	// Validate inputs
	if len(payload) == 0 {
		return "", ErrPayloadEmpty
	}
	if keySet == nil || len(keySet.Keys) == 0 {
		return "", ErrKeySetEmpty
	}

	// Find the public key in the key set
	var publicKey *jose.JSONWebKey
	if kid == "" {
		// Use the first public key if no KID specified
		for _, key := range keySet.Keys {
			if key.Use == UseTypeEnc && key.Algorithm == string(jose.ECDH_ES_A128KW) && key.IsPublic() {
				if key.KeyID == "" {
					return "", fmt.Errorf("public key is invalid: %w", ErrKIDMissing)
				}
				publicKey = &key
				break
			}
		}
	} else {
		// Find the specific key by KID
		for _, key := range keySet.Keys {
			if key.KeyID == kid && key.Use == UseTypeEnc && key.Algorithm == string(jose.ECDH_ES_A128KW) && key.IsPublic() {
				publicKey = &key
				break
			}
		}
	}
	if publicKey == nil {
		return "", ErrKeyNotFound
	}

	// Validate the public key
	if !publicKey.Valid() {
		return "", ErrKeyInvalid
	}

	// Create encrypter with ECDH-ES+A128KW key management and A128GCM content encryption
	encrypter, err := jose.NewEncrypter(
		jose.A128GCM,
		jose.Recipient{
			Algorithm: jose.ECDH_ES_A128KW,
			Key:       publicKey.Key,
			KeyID:     publicKey.KeyID,
		},
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrCreateEncrypter, err)
	}

	// Encrypt the payload
	jwe, err := encrypter.Encrypt(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrEncryptPayload, err)
	}

	// Return compact serialization
	return jwe.CompactSerialize()
}

// DecryptTokenWithKeySet decrypts a JWE token using ECDH-ES+A128KW with the provided key set.
// It extracts the private key from the key set using the KID from the JWE headers.
// Returns the decrypted payload or an error if the key cannot be found or decryption fails.
func DecryptTokenWithKeySet(jweData []byte, keySet *jose.JSONWebKeySet) ([]byte, error) {
	// Validate inputs
	if len(jweData) == 0 {
		return nil, ErrPayloadEmpty
	}
	if keySet == nil || len(keySet.Keys) == 0 {
		return nil, ErrKeySetEmpty
	}

	// Parse the JWE token
	jwe, err := jose.ParseEncrypted(string(jweData), []jose.KeyAlgorithm{jose.ECDH_ES_A128KW}, []jose.ContentEncryption{jose.A128GCM})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseJWE, err)
	}

	// Extract the KID from the JWE header
	kid := jwe.Header.KeyID
	if kid == "" {
		return nil, ErrKIDNotFoundInHeaders
	}

	// Find the private key in the key set using the KID
	var privateKey *jose.JSONWebKey
	for _, key := range keySet.Keys {
		if key.KeyID == kid && key.Use == UseTypeEnc && key.Algorithm == string(jose.ECDH_ES_A128KW) && !key.IsPublic() {
			privateKey = &key
			break
		}
	}
	if privateKey == nil {
		return nil, ErrKeyNotFound
	}

	// Validate the private key
	if !privateKey.Valid() {
		return nil, ErrKeyInvalid
	}

	// Decrypt the payload using the private key
	payload, err := jwe.Decrypt(privateKey.Key)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecryptPayload, err)
	}

	return payload, nil
}
