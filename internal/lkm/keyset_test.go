// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package lkm

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestNewPublicKeySet(t *testing.T) {
	g := NewWithT(t)

	keySet := NewPublicKeySet()

	g.Expect(keySet).ToNot(BeNil())
	g.Expect(keySet.Issuer).To(BeEmpty())
	g.Expect(keySet.Keys).To(BeEmpty())
}

func TestNewPrivateKeySet(t *testing.T) {
	g := NewWithT(t)

	issuer := "test-issuer"
	keySet := NewPrivateKeySet(issuer)

	g.Expect(keySet).ToNot(BeNil())
	g.Expect(keySet.Issuer).To(Equal(issuer))
	g.Expect(keySet.Keys).To(BeEmpty())
}

func TestEdKeySet_AddPublicKey(t *testing.T) {
	g := NewWithT(t)

	// Generate test keys
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully adds public key", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()

		err := keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keySet.Keys).To(HaveLen(1))
		g.Expect(keySet.Keys[0].KeyID).To(Equal("key1"))
		g.Expect(keySet.Keys[0].Key).To(Equal(publicKey))
		g.Expect(keySet.Keys[0].Algorithm).To(Equal("EdDSA"))
		g.Expect(keySet.Keys[0].Use).To(Equal("sig"))
	})

	t.Run("adds multiple public keys", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()

		err := keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet.AddPublicKey(publicKey2, "key2")
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(keySet.Keys).To(HaveLen(2))
		// Most recent key should be first (prepended)
		g.Expect(keySet.Keys[0].KeyID).To(Equal("key2"))
		g.Expect(keySet.Keys[1].KeyID).To(Equal("key1"))
	})

	t.Run("fails when adding duplicate key ID", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()

		err := keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet.AddPublicKey(publicKey2, "key1")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key with ID key1 already exists"))
	})

	t.Run("fails when issuer is set", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPrivateKeySet("test-issuer")

		err := keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("cannot add public key to EdKeySet with issuer set"))
	})
}

func TestEdKeySet_AddPrivateKey(t *testing.T) {
	g := NewWithT(t)

	// Generate test keys
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	_, privateKey2, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully adds private key", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPrivateKeySet("test-issuer")

		err := keySet.AddPrivateKey(privateKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keySet.Keys).To(HaveLen(1))
		g.Expect(keySet.Keys[0].KeyID).To(Equal("key1"))
		g.Expect(keySet.Keys[0].Key).To(Equal(privateKey))
		g.Expect(keySet.Keys[0].Algorithm).To(Equal("EdDSA"))
		g.Expect(keySet.Keys[0].Use).To(Equal("sig"))
	})

	t.Run("fails when issuer is not set", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()

		err := keySet.AddPrivateKey(privateKey, "key1")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("issuer must be set before adding a private key"))
	})

	t.Run("fails when adding second private key", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPrivateKeySet("test-issuer")

		err := keySet.AddPrivateKey(privateKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet.AddPrivateKey(privateKey2, "key2")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("EdKeySet already contains a private key"))
	})
}

func TestEdKeySet_ToJSON(t *testing.T) {
	g := NewWithT(t)

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	keySet := NewPublicKeySet()
	err = keySet.AddPublicKey(publicKey, "key1")
	g.Expect(err).ToNot(HaveOccurred())

	jsonData, err := keySet.ToJSON()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(jsonData).ToNot(BeEmpty())
	g.Expect(string(jsonData)).To(ContainSubstring("\"keys\""))
	g.Expect(string(jsonData)).To(ContainSubstring("\"key1\""))
}

func TestEdKeySet_WriteFile(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("writes public key set to file", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()
		err := keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "public_keys.json")
		err = keySet.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Check file exists and has correct permissions
		info, err := os.Stat(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0644)))

		// Check file content
		data, err := os.ReadFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(string(data)).To(ContainSubstring("\"key1\""))
	})

	t.Run("writes private key set to file", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPrivateKeySet("test-issuer")
		err := keySet.AddPrivateKey(privateKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "private_keys.json")
		err = keySet.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Check file exists and has correct permissions
		info, err := os.Stat(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
	})

	t.Run("prevents overwriting existing private key file", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPrivateKeySet("test-issuer")
		err := keySet.AddPrivateKey(privateKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "existing_private.json")
		// Create existing file
		err = os.WriteFile(filePath, []byte("existing"), 0600)
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet.WriteFile(filePath)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("already exists, refusing to overwrite"))
	})

	t.Run("fails to write empty key set", func(t *testing.T) {
		g := NewWithT(t)
		keySet := NewPublicKeySet()

		filePath := filepath.Join(tempDir, "empty.json")
		err := keySet.WriteFile(filePath)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("cannot write empty EdKeySet"))
	})

	t.Run("appends new keys to existing public key file", func(t *testing.T) {
		g := NewWithT(t)
		publicKey1, _, err := ed25519.GenerateKey(rand.Reader)
		g.Expect(err).ToNot(HaveOccurred())

		publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
		g.Expect(err).ToNot(HaveOccurred())

		// Create initial key set and write to file
		keySet1 := NewPublicKeySet()
		err = keySet1.AddPublicKey(publicKey1, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "append_test.json")
		err = keySet1.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Create second key set and append to existing file
		keySet2 := NewPublicKeySet()
		err = keySet2.AddPublicKey(publicKey2, "key2")
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet2.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Read the file and verify both keys are present
		finalKeySet, err := EdKeySetFromFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(finalKeySet.Keys).To(HaveLen(2))

		// Verify the new key is first (most recent)
		g.Expect(finalKeySet.Keys[0].KeyID).To(Equal("key2"))
		g.Expect(finalKeySet.Keys[1].KeyID).To(Equal("key1"))
	})

	t.Run("fails when appending duplicate key ID", func(t *testing.T) {
		g := NewWithT(t)
		publicKey1, _, err := ed25519.GenerateKey(rand.Reader)
		g.Expect(err).ToNot(HaveOccurred())

		publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
		g.Expect(err).ToNot(HaveOccurred())

		// Create initial key set and write to file
		keySet1 := NewPublicKeySet()
		err = keySet1.AddPublicKey(publicKey1, "duplicate-key")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "duplicate_test.json")
		err = keySet1.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Create second key set with same key ID
		keySet2 := NewPublicKeySet()
		err = keySet2.AddPublicKey(publicKey2, "duplicate-key")
		g.Expect(err).ToNot(HaveOccurred())

		// Attempt to append should fail
		err = keySet2.WriteFile(filePath)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("key with ID duplicate-key already exists"))
	})

	t.Run("handles corrupted existing file gracefully", func(t *testing.T) {
		g := NewWithT(t)
		publicKey, _, err := ed25519.GenerateKey(rand.Reader)
		g.Expect(err).ToNot(HaveOccurred())

		// Create a corrupted JSON file
		filePath := filepath.Join(tempDir, "corrupted.json")
		err = os.WriteFile(filePath, []byte("invalid json"), 0644)
		g.Expect(err).ToNot(HaveOccurred())

		// Try to append to corrupted file
		keySet := NewPublicKeySet()
		err = keySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		err = keySet.WriteFile(filePath)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to read existing key set"))
	})
}

func TestEdKeySetFromJSON(t *testing.T) {
	g := NewWithT(t)

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a valid key set and convert to JSON
	originalKeySet := NewPublicKeySet()
	err = originalKeySet.AddPublicKey(publicKey, "key1")
	g.Expect(err).ToNot(HaveOccurred())

	jsonData, err := originalKeySet.ToJSON()
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully parses valid JSON", func(t *testing.T) {
		g := NewWithT(t)
		keySet, err := EdKeySetFromJSON(jsonData)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keySet.Keys).To(HaveLen(1))
		g.Expect(keySet.Keys[0].KeyID).To(Equal("key1"))
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		g := NewWithT(t)
		_, err := EdKeySetFromJSON([]byte("invalid json"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to unmarshal EdKeySet"))
	})

	t.Run("fails on empty keys", func(t *testing.T) {
		g := NewWithT(t)
		emptyJSON := []byte(`{"keys": []}`)
		_, err := EdKeySetFromJSON(emptyJSON)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("EdKeySet has no keys"))
	})

	t.Run("fails on private key set with multiple keys", func(t *testing.T) {
		g := NewWithT(t)
		multiKeyJSON := []byte(`{"issuer": "test", "keys": [{"kty": "OKP", "crv": "Ed25519", "x": "test", "kid": "key1"}, {"kty": "OKP", "crv": "Ed25519", "x": "test", "kid": "key2"}]}`)
		_, err := EdKeySetFromJSON(multiKeyJSON)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("cannot contain multiple keys"))
	})
}

func TestEdKeySetFromFile(t *testing.T) {
	g := NewWithT(t)
	tempDir := t.TempDir()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully reads from file", func(t *testing.T) {
		g := NewWithT(t)
		// Create a key set file
		originalKeySet := NewPublicKeySet()
		err := originalKeySet.AddPublicKey(publicKey, "key1")
		g.Expect(err).ToNot(HaveOccurred())

		filePath := filepath.Join(tempDir, "test_keys.json")
		err = originalKeySet.WriteFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())

		// Read it back
		keySet, err := EdKeySetFromFile(filePath)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(keySet.Keys).To(HaveLen(1))
		g.Expect(keySet.Keys[0].KeyID).To(Equal("key1"))
	})

	t.Run("fails on non-existent file", func(t *testing.T) {
		g := NewWithT(t)
		_, err := EdKeySetFromFile(filepath.Join(tempDir, "nonexistent.json"))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to read EdKeySet from file"))
	})
}

func TestEdPublicKeyFromSet(t *testing.T) {
	g := NewWithT(t)

	publicKey1, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	publicKey2, _, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a key set with multiple keys
	keySet := NewPublicKeySet()
	err = keySet.AddPublicKey(publicKey1, "key1")
	g.Expect(err).ToNot(HaveOccurred())
	err = keySet.AddPublicKey(publicKey2, "key2")
	g.Expect(err).ToNot(HaveOccurred())

	jsonData, err := keySet.ToJSON()
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully extracts existing public key", func(t *testing.T) {
		g := NewWithT(t)
		edKey, err := EdPublicKeyFromSet(jsonData, "key1")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(edKey.KeyID).To(Equal("key1"))
		g.Expect(edKey.Key).To(Equal(publicKey1))
	})

	t.Run("fails on non-existent key ID", func(t *testing.T) {
		g := NewWithT(t)
		_, err := EdPublicKeyFromSet(jsonData, "nonexistent")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no public key found with ID nonexistent"))
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		g := NewWithT(t)
		_, err := EdPublicKeyFromSet([]byte("invalid"), "key1")
		g.Expect(err).To(HaveOccurred())
	})
}

func TestEdPrivateKeyFromSet(t *testing.T) {
	g := NewWithT(t)

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	// Create a private key set
	keySet := NewPrivateKeySet("test-issuer")
	err = keySet.AddPrivateKey(privateKey, "key1")
	g.Expect(err).ToNot(HaveOccurred())

	jsonData, err := keySet.ToJSON()
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("successfully extracts private key", func(t *testing.T) {
		g := NewWithT(t)
		edKey, err := EdPrivateKeyFromSet(jsonData)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(edKey.KeyID).To(Equal("key1"))
		g.Expect(edKey.Key).To(Equal(privateKey))
		g.Expect(edKey.Issuer).To(Equal("test-issuer"))
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		g := NewWithT(t)
		_, err := EdPrivateKeyFromSet([]byte("invalid"))
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("fails on empty key set", func(t *testing.T) {
		g := NewWithT(t)
		emptyJSON := []byte(`{"keys": []}`)
		_, err := EdPrivateKeyFromSet(emptyJSON)
		g.Expect(err).To(HaveOccurred())
	})
}
