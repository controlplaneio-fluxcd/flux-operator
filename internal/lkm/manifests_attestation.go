// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"golang.org/x/mod/sumdb/dirhash"
)

// ManifestsAttestation represents an attestation for manifests.
// It provides methods to sign, verify, and validate the claims of the attestation.
type ManifestsAttestation struct {
	att Attestation
}

// NewManifestsAttestation creates a new ManifestsAttestation instance.
func NewManifestsAttestation(audience string) *ManifestsAttestation {
	var audiences []string
	if audience != "" {
		audiences = []string{audience}
	}
	return &ManifestsAttestation{
		att: Attestation{
			Audience: audiences,
			Subject:  "manifests",
		},
	}
}

// GetAttestation returns the underlying Attestation object.
func (m *ManifestsAttestation) GetAttestation() Attestation {
	return m.att
}

// GetIssuedAt returns the timestamp when the underlying Attestation
// was issued, formatted as an RFC3339 string.
func (m *ManifestsAttestation) GetIssuedAt() string {
	return time.Unix(m.att.IssuedAt, 0).Format(time.RFC3339)
}

// GetChecksum returns the checksum of the ManifestsAttestation.
func (m *ManifestsAttestation) GetChecksum() string {
	if len(m.att.Digests) == 0 {
		return ""
	}
	return m.att.Digests[0]
}

// GetIssuer returns the issuer of the ManifestsAttestation.
func (m *ManifestsAttestation) GetIssuer() string {
	return m.att.Issuer
}

// ToJSON serializes the attestation to JSON format.
func (m *ManifestsAttestation) ToJSON() ([]byte, error) {
	data, err := json.Marshal(m.att)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal attestation: %w", err)
	}
	return data, nil
}

// Sign scans the specified directory recursively for files, calculates their checksum,
// and generates an attestation for the manifests.
// It returns the signed JWT token and the list of files included in the attestation.
func (m *ManifestsAttestation) Sign(privateKey *EdPrivateKey, dirPath string, ignore []string) (string, []string, error) {
	if privateKey == nil {
		return "", nil, ErrPrivateKeyRequired
	}
	if len(m.att.Digests) != 0 {
		return "", nil, ErrClaimChecksumImmutable
	}

	// Generate the license ID.
	jti, err := uuid.NewV6()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate UUID: %w", err)
	}

	// Compute the checksum of the directory contents.
	checksum, files, err := hashDir(dirPath, "", ignore, dirhash.Hash1)
	if err != nil {
		return "", nil, fmt.Errorf("failed to scan files in %s: %w", dirPath, err)
	}

	// Set the attestation fields.
	now := time.Now()
	m.att.ID = jti.String()
	m.att.Issuer = privateKey.Issuer
	m.att.IssuedAt = now.Unix()
	m.att.NotBefore = now.Unix()
	m.att.Expiry = now.AddDate(999, 0, 0).Unix()
	m.att.Digests = []string{checksum}

	// Validate the attestation.
	if err := m.att.Validate("", "manifests"); err != nil {
		return "", nil, err
	}

	// Marshal the claims to JSON
	payload, err := m.ToJSON()
	if err != nil {
		return "", nil, err
	}

	tokenString, err := GenerateSignedToken(payload, privateKey)
	if err != nil {
		return "", nil, err
	}

	return tokenString, files, nil
}

// Verify checks the signature of the JWT token, scans the specified directory,
// and verifies that the checksum matches the one in the attestation.
// It returns a list of files that were scanned and an error if verification fails.
func (m *ManifestsAttestation) Verify(jwtData []byte, publicKey *EdPublicKey, dirPath string, ignore []string) ([]string, error) {
	if publicKey == nil {
		return nil, ErrPublicKeyRequired
	}

	// Parse the JWT token to extract the claims and verify the signature.
	jws, err := jose.ParseSigned(string(jwtData), []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return nil, InvalidAttestationError(ErrParseToken)
	}
	payload, err := jws.Verify(publicKey.Key)
	if err != nil {
		return nil, InvalidAttestationError(ErrVerifySig)
	}

	var att Attestation
	if err := json.Unmarshal(payload, &att); err != nil {
		return nil, InvalidAttestationError(ErrParseClaims)
	}
	m.att = att

	// Validate the attestation.
	if err := m.att.Validate("", "manifests"); err != nil {
		return nil, err
	}

	// Compute the checksum of the directory contents.
	checksum, files, err := hashDir(dirPath, "", ignore, dirhash.Hash1)
	if err != nil {
		return nil, fmt.Errorf("failed to scan files in %s: %w", dirPath, err)
	}

	// Verify that the computed checksum matches the one in the attestation.
	if len(m.att.Digests) == 0 || checksum != m.att.Digests[0] {
		return nil, InvalidAttestationError(ErrClaimChecksumMismatch)
	}

	return files, nil
}

// hashDir returns the hash of the local file system directory dir,
// replacing the directory name itself with prefix in the file names
// used in the hash function.
func hashDir(dir, prefix string, ignore []string, hash dirhash.Hash) (string, []string, error) {
	files, err := dirFiles(dir, prefix, ignore)
	if err != nil {
		return "", nil, err
	}
	osOpen := func(name string) (io.ReadCloser, error) {
		return os.Open(filepath.Join(dir, strings.TrimPrefix(name, prefix)))
	}
	checksum, err := hash(files, osOpen)
	if err != nil {
		return "", nil, err
	}

	return checksum, files, nil
}

// dirFiles returns the list of files in the tree rooted at dir,
// replacing the directory name dir with prefix in each name.
// The resulting names always use forward slashes.
func dirFiles(dir, prefix string, ignore []string) ([]string, error) {
	var files []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		} else if file == dir {
			return fmt.Errorf("%s is not a directory", dir)
		}

		rel := file
		if dir != "." {
			rel = file[len(dir)+1:]
		}
		f := filepath.Join(prefix, rel)

		// Check if the file matches any ignore pattern
		for _, exclusion := range ignore {
			if strings.HasSuffix(f, exclusion) {
				// Skip files that match the ignore pattern
				return nil
			}
		}

		files = append(files, filepath.ToSlash(f))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
