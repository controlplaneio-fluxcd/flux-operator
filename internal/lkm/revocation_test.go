// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
)

func TestNewRevocationKeySet(t *testing.T) {
	g := NewWithT(t)

	issuer := "test-issuer"
	rks := NewRevocationKeySet(issuer)

	g.Expect(rks).ToNot(BeNil())
	g.Expect(rks.Issuer).To(Equal(issuer))
	g.Expect(rks.Keys).ToNot(BeNil())
	g.Expect(rks.Keys).To(BeEmpty())
}

func TestRevocationKeySet_AddKey(t *testing.T) {

	t.Run("adds valid UUID v6 key", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(rks.Keys).To(HaveKey(keyID.String()))

		timestamp, exists := rks.Keys[keyID.String()]
		g.Expect(exists).To(BeTrue())
		g.Expect(timestamp).To(BeNumerically(">", 0))
		g.Expect(timestamp).To(BeNumerically("~", time.Now().Unix(), 5))
	})

	t.Run("fails with invalid UUID", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		err := rks.AddKey("invalid-uuid")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid key ID"))
		g.Expect(rks.Keys).To(BeEmpty())
	})

	t.Run("fails with non-UUID v6", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		keyID := uuid.New() // This creates UUID v4
		err := rks.AddKey(keyID.String())
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("must be a UUID v6"))
		g.Expect(rks.Keys).To(BeEmpty())
	})

	t.Run("updates timestamp for existing key", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		firstTimestamp := rks.Keys[keyID.String()]

		time.Sleep(1 * time.Second)

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		secondTimestamp := rks.Keys[keyID.String()]
		g.Expect(secondTimestamp).To(BeNumerically(">", firstTimestamp))
	})
}

func TestRevocationKeySet_IsRevoked(t *testing.T) {

	t.Run("returns false for nil license", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		revoked, timestamp := rks.IsRevoked(nil)
		g.Expect(revoked).To(BeFalse())
		g.Expect(timestamp).To(Equal("license is nil"))
	})

	t.Run("returns false for non-revoked license", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		license, err := NewLicense("test-issuer", "test-subject", "test-audience", 1*time.Hour, nil)
		g.Expect(err).ToNot(HaveOccurred())

		revoked, timestamp := rks.IsRevoked(license)
		g.Expect(revoked).To(BeFalse())
		g.Expect(timestamp).To(BeEmpty())
	})

	t.Run("returns true for revoked license with formatted timestamp", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		license, err := NewLicense("test-issuer", "test-subject", "test-audience", 1*time.Hour, nil)
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(license.GetKey().ID)
		g.Expect(err).ToNot(HaveOccurred())

		revoked, timestampStr := rks.IsRevoked(license)
		g.Expect(revoked).To(BeTrue())
		g.Expect(timestampStr).ToNot(BeEmpty())

		// Verify timestamp is in RFC3339 format
		parsedTime, err := time.Parse(time.RFC3339, timestampStr)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsedTime).To(BeTemporally("~", time.Now(), 10*time.Second))
	})
}

func TestRevocationKeySet_ToJSON(t *testing.T) {

	t.Run("serializes empty set", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		data, err := rks.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())

		var parsed RevocationKeySet
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed.Issuer).To(Equal("test-issuer"))
		g.Expect(parsed.Keys).To(BeEmpty())
	})

	t.Run("serializes set with keys", func(t *testing.T) {
		g := NewWithT(t)
		rks := NewRevocationKeySet("test-issuer")

		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		data, err := rks.ToJSON()
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(data).ToNot(BeEmpty())

		var parsed RevocationKeySet
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed.Issuer).To(Equal("test-issuer"))
		g.Expect(parsed.Keys).To(HaveKey(keyID.String()))
		g.Expect(parsed.Keys[keyID.String()]).To(Equal(rks.Keys[keyID.String()]))
	})
}

func TestRevocationKeySet_WriteFile(t *testing.T) {

	t.Run("writes new file successfully", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "revocation.json")

		rks := NewRevocationKeySet("test-issuer")
		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify file was created
		g.Expect(filename).To(BeAnExistingFile())

		// Verify file contents
		data, err := os.ReadFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		var parsed RevocationKeySet
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed.Issuer).To(Equal("test-issuer"))
		g.Expect(parsed.Keys).To(HaveKey(keyID.String()))
	})

	t.Run("merges with existing file", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "revocation.json")

		// Create initial revocation set and write to file
		rks1 := NewRevocationKeySet("test-issuer")
		keyID1, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks1.AddKey(keyID1.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks1.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		// Create second revocation set with different key
		rks2 := NewRevocationKeySet("test-issuer")
		keyID2, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks2.AddKey(keyID2.String())
		g.Expect(err).ToNot(HaveOccurred())

		// Write second set to same file
		err = rks2.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify file contains both keys
		data, err := os.ReadFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		var parsed RevocationKeySet
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed.Issuer).To(Equal("test-issuer"))
		g.Expect(parsed.Keys).To(HaveKey(keyID1.String()))
		g.Expect(parsed.Keys).To(HaveKey(keyID2.String()))
		g.Expect(parsed.Keys).To(HaveLen(2))
	})

	t.Run("updates timestamp when merging duplicate keys", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "revocation.json")

		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		// Create initial revocation set
		rks1 := NewRevocationKeySet("test-issuer")
		err = rks1.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks1.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		originalTimestamp := rks1.Keys[keyID.String()]

		time.Sleep(1 * time.Second)

		// Create second revocation set with same key
		rks2 := NewRevocationKeySet("test-issuer")
		err = rks2.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks2.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		// Verify timestamp was updated
		data, err := os.ReadFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		var parsed RevocationKeySet
		err = json.Unmarshal(data, &parsed)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(parsed.Keys).To(HaveKey(keyID.String()))
		g.Expect(parsed.Keys[keyID.String()]).To(BeNumerically(">", originalTimestamp))
	})

	t.Run("fails with issuer mismatch", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "revocation.json")

		// Create initial revocation set
		rks1 := NewRevocationKeySet("issuer-1")
		keyID1, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks1.AddKey(keyID1.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks1.WriteFile(filename)
		g.Expect(err).ToNot(HaveOccurred())

		// Try to write different issuer
		rks2 := NewRevocationKeySet("issuer-2")
		keyID2, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks2.AddKey(keyID2.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks2.WriteFile(filename)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("issuer mismatch"))
		g.Expect(err.Error()).To(ContainSubstring("issuer-1"))
		g.Expect(err.Error()).To(ContainSubstring("issuer-2"))
	})

	t.Run("fails to read corrupted existing file", func(t *testing.T) {
		g := NewWithT(t)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "revocation.json")

		// Create corrupted file
		err := os.WriteFile(filename, []byte("invalid json"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		rks := NewRevocationKeySet("test-issuer")
		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.AddKey(keyID.String())
		g.Expect(err).ToNot(HaveOccurred())

		err = rks.WriteFile(filename)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to parse existing revocation file"))
	})
}

func TestRevocationKeySetFromJSON(t *testing.T) {

	t.Run("deserializes valid JSON", func(t *testing.T) {
		g := NewWithT(t)
		keyID, err := uuid.NewV6()
		g.Expect(err).ToNot(HaveOccurred())

		timestamp := time.Now().Unix()
		jsonStr := fmt.Sprintf(`{
			"issuer": "test-issuer",
			"keys": {
				"%s": %d
			}
		}`, keyID.String(), timestamp)

		rks, err := RevocationKeySetFromJSON([]byte(jsonStr))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(rks).ToNot(BeNil())
		g.Expect(rks.Issuer).To(Equal("test-issuer"))
		g.Expect(rks.Keys).To(HaveKey(keyID.String()))
		g.Expect(rks.Keys[keyID.String()]).To(Equal(timestamp))
	})

	t.Run("deserializes empty keys", func(t *testing.T) {
		g := NewWithT(t)
		jsonData := []byte(`{
			"issuer": "test-issuer",
			"keys": {}
		}`)

		rks, err := RevocationKeySetFromJSON(jsonData)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(rks).ToNot(BeNil())
		g.Expect(rks.Issuer).To(Equal("test-issuer"))
		g.Expect(rks.Keys).To(BeEmpty())
	})

	t.Run("fails with invalid JSON", func(t *testing.T) {
		g := NewWithT(t)
		jsonData := []byte(`invalid json`)

		rks, err := RevocationKeySetFromJSON(jsonData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(rks).To(BeNil())
	})

	t.Run("fails with missing issuer", func(t *testing.T) {
		g := NewWithT(t)
		jsonData := []byte(`{
			"keys": {}
		}`)

		rks, err := RevocationKeySetFromJSON(jsonData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("missing issuer"))
		g.Expect(rks).To(BeNil())
	})

	t.Run("fails with empty issuer", func(t *testing.T) {
		g := NewWithT(t)
		jsonData := []byte(`{
			"issuer": "",
			"keys": {}
		}`)

		rks, err := RevocationKeySetFromJSON(jsonData)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("missing issuer"))
		g.Expect(rks).To(BeNil())
	})
}
