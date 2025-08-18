// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"

	"github.com/go-jose/go-jose/v4"
)

// EdKeySet represents a JWK Set object for holding Ed25519 public or private keys.
type EdKeySet struct {
	// Issuer is the identifier of the entity that issued the keys.
	// It should be present when the set contains a private key.
	// If the set contains only public keys, this field must be empty.
	Issuer string `json:"issuer,omitempty"`
	// Keys is a list of JSON Web Keys (JWKs) that make up the set.
	Keys []jose.JSONWebKey `json:"keys"`
}

// EdPublicKey is an envelope for an Ed25519 public key,
// and its key ID.
type EdPublicKey struct {
	// Key is the Ed25519 public key.
	Key ed25519.PublicKey

	// KeyID is the unique identifier for the key.
	KeyID string
}

// EdPrivateKey is an envelope for an Ed25519 private key,
// including its key ID and issuer information.
type EdPrivateKey struct {
	// Key is the Ed25519 private key.
	Key ed25519.PrivateKey

	// KeyID is the unique identifier for the key.
	KeyID string

	// Issuer is the identifier of the entity that issued the key.
	Issuer string
}

// NewPublicKeySet creates a new EdKeySet for holding public keys.
func NewPublicKeySet() *EdKeySet {
	return &EdKeySet{
		Keys: []jose.JSONWebKey{},
	}
}

// NewPrivateKeySet creates a new EdKeySet for holding a private key.
func NewPrivateKeySet(issuer string) *EdKeySet {
	return &EdKeySet{
		Issuer: issuer,
		Keys:   []jose.JSONWebKey{},
	}
}

// AddPublicKey adds a new Ed25519 key to the EdKeySet.
// The key set is designed to hold multiple public keys.
func (k *EdKeySet) AddPublicKey(key ed25519.PublicKey, keyID string) error {
	if k.Issuer != "" {
		return fmt.Errorf("cannot add public key to EdKeySet with issuer set")
	}

	for _, existingKey := range k.Keys {
		if existingKey.KeyID == keyID {
			return fmt.Errorf("key with ID %s already exists in the set", keyID)
		}
	}

	jwk := jose.JSONWebKey{
		Key:       key,
		KeyID:     keyID,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}

	// Prepend the key to the set to ensure the most recent key is first.
	k.Keys = append([]jose.JSONWebKey{jwk}, k.Keys...)
	return nil
}

// AddPrivateKey adds a new Ed25519 private key to the EdKeySet.
// The key set is designed to hold a single private key.
func (k *EdKeySet) AddPrivateKey(key ed25519.PrivateKey, keyID string) error {
	if k.Issuer == "" {
		return fmt.Errorf("issuer must be set before adding a private key")
	}

	if len(k.Keys) > 0 {
		return fmt.Errorf("EdKeySet already contains a private key, cannot add another")
	}

	jwk := jose.JSONWebKey{
		Key:       key,
		KeyID:     keyID,
		Algorithm: string(jose.EdDSA),
		Use:       "sig",
	}

	k.Keys = append(k.Keys, jwk)
	return nil
}

// ToJSON converts the EdKeySet to a JSON byte slice.
func (k *EdKeySet) ToJSON() ([]byte, error) {
	return json.MarshalIndent(*k, "", "  ")
}

// WriteFile writes the EdKeySet to the specified file in JSON format.
// If the set contains a private key, it restricts permissions to owner only (0600).
// If the set contains only public keys, it allows read/write for owner and read for others (0644).
// If the file already exists and contains public keys, it appends the new keys to the existing set,
// ensuring no duplicate key IDs are added.
func (k *EdKeySet) WriteFile(filePath string) error {
	if len(k.Keys) == 0 {
		return fmt.Errorf("cannot write empty EdKeySet to file")
	}

	data, err := k.ToJSON()
	if err != nil {
		return err
	}

	// Default permissions for the public key set
	// is readable by everyone, writable by owner (0644).
	perm := os.FileMode(0644)

	if k.Issuer != "" {
		// If the issuer is set we assume this is a private key set
		// and restrict permissions to the owner only (0600).
		perm = os.FileMode(0600)

		// Prevent overwriting existing file with private key set.
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			return fmt.Errorf("file %s already exists, refusing to overwrite", filePath)
		}
	} else {
		// For public key set, if the file exists, we append the keys to the existing file.
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			existingKeySet, err := EdKeySetFromFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read existing key set from file %s: %w", filePath, err)
			}

			if existingKeySet.Issuer != "" {
				return fmt.Errorf("file %s contains a private key set, cannot append public keys", filePath)
			}

			// Check for duplicate key IDs before merging.
			for _, newKey := range k.Keys {
				for _, existingKey := range existingKeySet.Keys {
					if existingKey.KeyID == newKey.KeyID {
						return fmt.Errorf("key with ID %s already exists in file %s", newKey.KeyID, filePath)
					}
				}
			}

			// Merge keys from current set into existing set.
			// Prepend new keys to ensure the most recent keys are first.
			existingKeySet.Keys = append(k.Keys, existingKeySet.Keys...)

			// Serialize the merged key set to JSON.
			data, err = existingKeySet.ToJSON()
			if err != nil {
				return err
			}
		}
	}

	return os.WriteFile(filePath, data, perm)
}

// EdKeySetFromJSON creates an EdKeySet from a JSON byte slice.
func EdKeySetFromJSON(data []byte) (*EdKeySet, error) {
	var keySet EdKeySet
	if err := json.Unmarshal(data, &keySet); err != nil {
		return nil, fmt.Errorf("failed to unmarshal EdKeySet: %w", err)
	}
	if len(keySet.Keys) == 0 {
		return nil, fmt.Errorf("EdKeySet has no keys")
	}

	if keySet.Issuer != "" && len(keySet.Keys) > 1 {
		return nil, fmt.Errorf("EdKeySet with issuer %s cannot contain multiple keys", keySet.Issuer)
	}

	return &keySet, nil
}

// EdKeySetFromFile reads an EdKeySet from a JSON file.
func EdKeySetFromFile(filePath string) (*EdKeySet, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read EdKeySet from file %s: %w", filePath, err)
	}
	return EdKeySetFromJSON(data)
}

// EdPublicKeyFromSet extracts the Ed25519 public key by key ID
// from a byte slice representing an EdKeySet in JSON format.
func EdPublicKeyFromSet(data []byte, keyID string) (*EdPublicKey, error) {
	keySet, err := EdKeySetFromJSON(data)
	if err != nil {
		return nil, err
	}

	for _, key := range keySet.Keys {
		if key.KeyID == keyID {
			if key.Algorithm != string(jose.EdDSA) {
				return nil, fmt.Errorf("key with ID %s has unsupported algorithm %s, expected %s", keyID, key.Algorithm, jose.EdDSA)
			}
			if key.Use != "sig" {
				return nil, fmt.Errorf("key with ID %s has unsupported use %s, expected 'sig'", keyID, key.Use)
			}

			publicKey, ok := key.Key.(ed25519.PublicKey)
			if !ok {
				return nil, fmt.Errorf("key with ID %s is not an Ed25519 public key", keyID)
			}

			return &EdPublicKey{
				Key:   publicKey,
				KeyID: key.KeyID,
			}, nil
		}
	}

	return nil, fmt.Errorf("no public key found with ID %s", keyID)
}

// EdPrivateKeyFromSet extracts the first Ed25519 private key
// from a byte slice representing an EdKeySet in JSON format.
func EdPrivateKeyFromSet(data []byte) (*EdPrivateKey, error) {
	keySet, err := EdKeySetFromJSON(data)
	if err != nil {
		return nil, err
	}

	if len(keySet.Keys) == 0 {
		return nil, fmt.Errorf("no keys found in set")
	}

	firstKey := keySet.Keys[0]
	if firstKey.KeyID == "" {
		return nil, fmt.Errorf("key ID is missing")
	}

	if firstKey.Algorithm != string(jose.EdDSA) {
		return nil, fmt.Errorf("key has unsupported algorithm %s, expected %s", firstKey.Algorithm, jose.EdDSA)
	}

	if firstKey.Use != "sig" {
		return nil, fmt.Errorf("key has unsupported use %s, expected 'sig'", firstKey.Use)
	}

	privateKey, ok := firstKey.Key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an Ed25519 private key")
	}

	return &EdPrivateKey{
		Key:    privateKey,
		KeyID:  firstKey.KeyID,
		Issuer: keySet.Issuer,
	}, nil
}
