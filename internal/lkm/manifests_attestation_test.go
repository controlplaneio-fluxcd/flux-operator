// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

// testManifestAttestation returns a valid Attestation for testing
func testManifestAttestation() Attestation {
	now := time.Now()
	return Attestation{
		ID:        "01f080cb-8881-6194-a0de-c69c5184ad4d",
		Issuer:    "test-issuer",
		Subject:   "manifests",
		Audience:  []string{"test-audience"},
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		Expiry:    now.AddDate(999, 0, 0).Unix(),
		Digests:   []string{"h1:test-checksum+hash"},
	}
}

// createTestDirectory creates a temporary directory with test files
func createTestDirectory(t *testing.T) string {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create some test files
	err := os.WriteFile(filepath.Join(tmpDir, "test1.yaml"), []byte("content1"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(tmpDir, "test2.yaml"), []byte("content2"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	// Create subdirectory with file
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	err = os.WriteFile(filepath.Join(subDir, "test3.yaml"), []byte("content3"), 0644)
	g.Expect(err).ToNot(HaveOccurred())

	return tmpDir
}

func TestNewManifestsAttestation(t *testing.T) {
	t.Run("creates attestation with valid audience", func(t *testing.T) {
		g := NewWithT(t)

		audience := "test-audience"
		ma := NewManifestsAttestation(audience)

		g.Expect(ma).ToNot(BeNil())
		att := ma.GetAttestation()
		g.Expect(att.Audience).To(Equal([]string{audience}))
		g.Expect(att.Subject).To(Equal("manifests"))
		// Other fields should be empty initially
		g.Expect(att.ID).To(BeEmpty())
		g.Expect(att.Issuer).To(BeEmpty())
		g.Expect(att.IssuedAt).To(BeZero())
		g.Expect(att.Digests).To(BeEmpty())
	})

	t.Run("creates attestation with empty audience", func(t *testing.T) {
		g := NewWithT(t)

		ma := NewManifestsAttestation("")

		g.Expect(ma).ToNot(BeNil())
		att := ma.GetAttestation()
		g.Expect(att.Audience).To(BeEmpty())
		g.Expect(att.Subject).To(Equal("manifests"))
	})
}

func TestManifestsAttestation_GetAttestation(t *testing.T) {
	g := NewWithT(t)

	att := testManifestAttestation()
	ma := &ManifestsAttestation{att: att}

	result := ma.GetAttestation()
	g.Expect(result).To(Equal(att))
}

func TestManifestsAttestation_ToJSON(t *testing.T) {
	t.Run("serializes attestation to JSON", func(t *testing.T) {
		g := NewWithT(t)
		att := testManifestAttestation()
		ma := &ManifestsAttestation{att: att}

		data, err := ma.ToJSON()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())

		// Verify it can be unmarshaled back
		var parsed Attestation
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed).To(Equal(att))
	})

	t.Run("handles empty attestation", func(t *testing.T) {
		g := NewWithT(t)
		ma := &ManifestsAttestation{}

		data, err := ma.ToJSON()

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())

		var parsed Attestation
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
	})
}

func TestManifestsAttestation_Sign(t *testing.T) {
	t.Run("successfully signs directory", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		_, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, files, err := ma.Sign(privateKey, testDir, nil)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(strings.Count(token, ".")).To(Equal(2)) // JWT has 3 parts separated by dots
		g.Expect(files).To(HaveLen(3))                   // test1.yaml, test2.yaml, subdir/test3.yaml

		// Verify attestation was populated
		att := ma.GetAttestation()
		g.Expect(att.ID).ToNot(BeEmpty())
		g.Expect(att.Issuer).To(Equal(privateKey.Issuer))
		g.Expect(att.Subject).To(Equal("manifests"))
		g.Expect(att.Audience).To(Equal([]string{"test-audience"}))
		g.Expect(att.IssuedAt).To(BeNumerically(">", 0))
		g.Expect(att.Digests).ToNot(BeEmpty())
		g.Expect(att.Digests).To(HaveLen(1))

		// Verify ID is a valid UUID v6
		parsedUUID, err := uuid.Parse(att.ID)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedUUID.Version()).To(Equal(uuid.Version(6)))
	})

	t.Run("signs directory with ignore patterns", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		_, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		// Create a file to ignore
		err := os.WriteFile(filepath.Join(testDir, "ignore-me.txt"), []byte("ignored"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		ignore := []string{".txt"}
		token, files, err := ma.Sign(privateKey, testDir, ignore)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(token).ToNot(BeEmpty())
		g.Expect(files).To(HaveLen(3)) // Only .yaml files, .txt ignored

		// Verify ignored file is not in the list
		for _, file := range files {
			g.Expect(file).ToNot(ContainSubstring(".txt"))
		}
	})

	t.Run("fails with nil private key", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		testDir := createTestDirectory(t)

		token, files, err := ma.Sign(nil, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(files).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("private key is required"))
	})

	t.Run("fails when attestation already scanned", func(t *testing.T) {
		g := NewWithT(t)
		att := testManifestAttestation()
		ma := &ManifestsAttestation{att: att}
		_, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, files, err := ma.Sign(privateKey, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(files).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimChecksumImmutable)).To(BeTrue())
	})

	t.Run("fails with non-existent directory", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		_, privateKey := genTestKeys(t)

		token, files, err := ma.Sign(privateKey, "/non-existent-dir", nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(token).To(BeEmpty())
		g.Expect(files).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to scan files"))
	})
}

func TestManifestsAttestation_Verify(t *testing.T) {
	t.Run("successfully verifies valid attestation", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		// First sign to create attestation
		token, originalFiles, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		// Now verify
		ma2 := NewManifestsAttestation("test-audience")
		verifiedFiles, err := ma2.Verify([]byte(token), publicKey, testDir, nil)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(verifiedFiles).To(Equal(originalFiles))

		// Verify attestation was loaded correctly
		att := ma2.GetAttestation()
		originalAtt := ma.GetAttestation()
		g.Expect(att).To(Equal(originalAtt))
	})

	t.Run("verifies with ignore patterns", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		// Create a file to ignore
		err := os.WriteFile(filepath.Join(testDir, "ignore-me.txt"), []byte("ignored"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		ignore := []string{".txt"}
		token, _, err := ma.Sign(privateKey, testDir, ignore)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify with same ignore pattern
		ma2 := NewManifestsAttestation("test-audience")
		files, err := ma2.Verify([]byte(token), publicKey, testDir, ignore)

		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(files).To(HaveLen(3)) // Only .yaml files
	})

	t.Run("fails with nil public key", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		testDir := createTestDirectory(t)

		files, err := ma.Verify([]byte("token"), nil, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("public key is required"))
	})

	t.Run("fails with invalid token", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, _ := genTestKeys(t)
		testDir := createTestDirectory(t)

		files, err := ma.Verify([]byte("invalid-token"), publicKey, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(errors.Is(err, ErrParseToken)).To(BeTrue())
	})

	t.Run("fails with wrong public key", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		_, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		// Use different public key
		wrongPublicKey, _ := genTestKeys(t)

		ma2 := NewManifestsAttestation("test-audience")
		files, err := ma2.Verify([]byte(token), wrongPublicKey, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(errors.Is(err, ErrVerifySig)).To(BeTrue())
	})

	t.Run("fails when directory contents changed", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		// Modify directory contents
		err = os.WriteFile(filepath.Join(testDir, "new-file.yaml"), []byte("new content"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		ma2 := NewManifestsAttestation("test-audience")
		files, err := ma2.Verify([]byte(token), publicKey, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimChecksumMismatch)).To(BeTrue())
	})

	t.Run("fails when file content changed", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		// Modify existing file content
		err = os.WriteFile(filepath.Join(testDir, "test1.yaml"), []byte("modified content"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		ma2 := NewManifestsAttestation("test-audience")
		files, err := ma2.Verify([]byte(token), publicKey, testDir, nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(errors.Is(err, ErrClaimChecksumMismatch)).To(BeTrue())
	})

	t.Run("fails with non-existent directory", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		ma2 := NewManifestsAttestation("test-audience")
		files, err := ma2.Verify([]byte(token), publicKey, "/non-existent-dir", nil)

		g.Expect(err).To(HaveOccurred())
		g.Expect(files).To(BeNil())
		g.Expect(err.Error()).To(ContainSubstring("failed to scan files"))
	})

}

func TestManifestsAttestation_GetChecksum(t *testing.T) {
	t.Run("returns checksum from populated attestation", func(t *testing.T) {
		g := NewWithT(t)
		att := testManifestAttestation()
		ma := &ManifestsAttestation{att: att}

		checksum := ma.GetChecksum()
		g.Expect(checksum).To(Equal(att.Digests[0]))
		g.Expect(checksum).To(Equal("h1:test-checksum+hash"))
	})

	t.Run("returns checksum from signed attestation", func(t *testing.T) {
		g := NewWithT(t)
		ma := NewManifestsAttestation("test-audience")
		_, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		// Sign the attestation to populate checksum
		_, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		checksum := ma.GetChecksum()
		g.Expect(checksum).ToNot(BeEmpty())
		g.Expect(checksum).To(HavePrefix("h1:")) // Checksum should follow dirhash format
	})

	t.Run("returns checksum from verified attestation", func(t *testing.T) {
		g := NewWithT(t)
		// Create and sign attestation
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())
		originalChecksum := ma.GetChecksum()

		// Verify attestation and check checksum
		ma2 := NewManifestsAttestation("test-audience")
		_, err = ma2.Verify([]byte(token), publicKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		verifiedChecksum := ma2.GetChecksum()
		g.Expect(verifiedChecksum).To(Equal(originalChecksum))
		g.Expect(verifiedChecksum).ToNot(BeEmpty())
	})

	t.Run("returns empty string for uninitialized attestation", func(t *testing.T) {
		g := NewWithT(t)
		ma := &ManifestsAttestation{}

		checksum := ma.GetChecksum()
		g.Expect(checksum).To(BeEmpty())
	})
}

func TestManifestsAttestation_GetIssuer(t *testing.T) {
	t.Run("returns issuer from populated attestation", func(t *testing.T) {
		g := NewWithT(t)
		att := testManifestAttestation()
		ma := &ManifestsAttestation{att: att}

		issuer := ma.GetIssuer()
		g.Expect(issuer).To(Equal(att.Issuer))
		g.Expect(issuer).To(Equal("test-issuer"))
	})

	t.Run("returns issuer from verified attestation", func(t *testing.T) {
		g := NewWithT(t)
		// Create and sign attestation
		ma := NewManifestsAttestation("test-audience")
		publicKey, privateKey := genTestKeys(t)
		testDir := createTestDirectory(t)

		token, _, err := ma.Sign(privateKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())
		originalIssuer := ma.GetIssuer()

		// Verify attestation and check issuer
		ma2 := NewManifestsAttestation("test-audience")
		_, err = ma2.Verify([]byte(token), publicKey, testDir, nil)
		g.Expect(err).ToNot(HaveOccurred())

		verifiedIssuer := ma2.GetIssuer()
		g.Expect(verifiedIssuer).To(Equal(originalIssuer))
		g.Expect(verifiedIssuer).To(Equal("test-issuer"))
	})

	t.Run("returns empty string for uninitialized attestation", func(t *testing.T) {
		g := NewWithT(t)
		ma := &ManifestsAttestation{}

		issuer := ma.GetIssuer()
		g.Expect(issuer).To(BeEmpty())
	})
}
