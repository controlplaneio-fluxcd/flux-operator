// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// RevocationKeySet represents a set of license keys that have been revoked.
type RevocationKeySet struct {
	// Issuer is the identifier of the entity
	// that issued the revocation license keys.
	Issuer string `json:"issuer"`

	// Keys is a map of license key IDs and their revocation unix timestamps.
	Keys map[string]int64 `json:"keys"`
}

// NewRevocationKeySet creates a new RevocationKeySet for holding revoked license keys.
func NewRevocationKeySet(issuer string) *RevocationKeySet {
	return &RevocationKeySet{
		Issuer: issuer,
		Keys:   make(map[string]int64),
	}
}

// AddKey adds a license key ID and its revocation timestamp to the set.
func (r *RevocationKeySet) AddKey(keyID string) error {
	parsedUUID, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid key ID %q: %w", keyID, err)
	}
	if parsedUUID.Version() != uuid.Version(6) {
		return fmt.Errorf("key ID %q must be a UUID v6", keyID)
	}

	r.Keys[keyID] = time.Now().Unix()
	return nil
}

// IsRevoked checks if a License is present in the revocation set.
// It returns true if the license is revoked, along with the
// revocation timestamp in RFC3339 format.
func (r *RevocationKeySet) IsRevoked(lic *License) (bool, string) {
	if lic == nil {
		return false, "license is nil"
	}

	ts, exists := r.Keys[lic.GetKey().ID]
	if !exists {
		return false, ""
	}
	return exists, time.Unix(ts, 0).Format(time.RFC3339)
}

// ToJSON serializes the RevocationKeySet to JSON format.
func (r *RevocationKeySet) ToJSON() ([]byte, error) {
	return json.MarshalIndent(*r, "", "  ")
}

// WriteFile writes the RevocationKeySet to a file in JSON format.
// If the file already exists, the keys are merged with the existing set.
func (r *RevocationKeySet) WriteFile(filename string) error {
	toWrite := r

	// Check if file exists and merge if it does
	if _, err := os.Stat(filename); err == nil {
		// File exists, read and merge
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read existing revocation file: %w", err)
		}

		existing, err := RevocationKeySetFromJSON(data)
		if err != nil {
			return fmt.Errorf("failed to parse existing revocation file: %w", err)
		}

		// Verify issuers match
		if existing.Issuer != r.Issuer {
			return fmt.Errorf("issuer mismatch: existing %q, current %q", existing.Issuer, r.Issuer)
		}

		// Merge keys (current keys take precedence for timestamp updates)
		for keyID, timestamp := range r.Keys {
			existing.Keys[keyID] = timestamp
		}

		// Use the merged set for writing
		toWrite = existing
	}

	// Serialize to JSON
	data, err := toWrite.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize revocation set: %w", err)
	}

	// Write to file with appropriate permissions
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write revocation file: %w", err)
	}

	return nil
}

// RevocationKeySetFromJSON deserializes a JSON byte slice into a RevocationKeySet.
// It returns an error if the JSON is invalid or if the issuer is missing.
func RevocationKeySetFromJSON(data []byte) (*RevocationKeySet, error) {
	var rks RevocationKeySet
	if err := json.Unmarshal(data, &rks); err != nil {
		return nil, err
	}
	if rks.Issuer == "" {
		return nil, fmt.Errorf("missing issuer")
	}
	return &rks, nil
}
