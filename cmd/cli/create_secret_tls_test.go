// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

// generateSelfSignedCert generates a PEM-encoded self-signed certificate and key for testing.
func generateSelfSignedCert(g Gomega) (certPEM, keyPEM []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	g.Expect(err).ToNot(HaveOccurred())

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	g.Expect(err).ToNot(HaveOccurred())
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func TestCreateSecretTLSCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-tls")
	gt.Expect(err).ToNot(HaveOccurred())

	// Generate a self-signed cert+key pair for testing.
	tmpDir := t.TempDir()
	tlsCrt, tlsKey := generateSelfSignedCert(gt)
	tlsCrtFile := filepath.Join(tmpDir, "tls.crt")
	gt.Expect(os.WriteFile(tlsCrtFile, tlsCrt, 0600)).To(Succeed())
	tlsKeyFile := filepath.Join(tmpDir, "tls.key")
	gt.Expect(os.WriteFile(tlsKeyFile, tlsKey, 0600)).To(Succeed())
	caCrtFile := filepath.Join(tmpDir, "ca.crt")
	gt.Expect(os.WriteFile(caCrtFile, tlsCrt, 0600)).To(Succeed())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name: "create tls secret with ca only",
			args: []string{
				"create", "secret", "tls", "test-secret-ca",
				"--ca-crt-file=" + caCrtFile,
			},
		},
		{
			name: "create tls secret with cert and key",
			args: []string{
				"create", "secret", "tls", "test-secret-certkey",
				"--tls-crt-file=" + tlsCrtFile,
				"--tls-key-file=" + tlsKeyFile,
			},
		},
		{
			name: "create tls secret with all certs",
			args: []string{
				"create", "secret", "tls", "test-secret-full",
				"--tls-crt-file=" + tlsCrtFile,
				"--tls-key-file=" + tlsKeyFile,
				"--ca-crt-file=" + caCrtFile,
			},
		},
		{
			name: "create tls secret with export",
			args: []string{
				"create", "secret", "tls", "test-secret",
				"--ca-crt-file=" + caCrtFile,
				"--export",
			},
			expectExport: true,
		},
		{
			name: "create tls secret with annotations and labels",
			args: []string{
				"create", "secret", "tls", "test-secret-meta",
				"--ca-crt-file=" + caCrtFile,
				"--annotation=test.io/annotation=value",
				"--label=test.io/label=value",
			},
		},
		{
			name: "create immutable tls secret",
			args: []string{
				"create", "secret", "tls", "test-secret-immutable",
				"--ca-crt-file=" + caCrtFile,
				"--immutable",
			},
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "tls", "--ca-crt-file=" + caCrtFile},
			expectError: true,
		},
		{
			name: "missing ca cert file",
			args: []string{
				"create", "secret", "tls", "test-secret",
				"--ca-crt-file=/nonexistent/ca.crt",
				"--export",
			},
			expectError:  true,
			expectExport: true,
		},
		{
			name: "missing tls key file",
			args: []string{
				"create", "secret", "tls", "test-secret",
				"--tls-crt-file=" + tlsCrtFile,
				"--tls-key-file=/nonexistent/tls.key",
				"--export",
			},
			expectError:  true,
			expectExport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			if tt.expectExport {
				obj := &unstructured.Unstructured{}
				err = yaml.Unmarshal([]byte(output), &obj.Object)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(obj.GetAPIVersion()).To(Equal("v1"))
				g.Expect(obj.GetKind()).To(Equal("Secret"))
				g.Expect(obj.GetName()).To(Equal("test-secret"))
				g.Expect(obj.GetNamespace()).To(Equal(ns.Name))

				_, found, err := unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				secretName := "test-secret-ca"
				switch tt.name {
				case "create tls secret with cert and key":
					secretName = "test-secret-certkey"
				case "create tls secret with all certs":
					secretName = "test-secret-full"
				case "create tls secret with annotations and labels":
					secretName = "test-secret-meta"
				case "create immutable tls secret":
					secretName = "test-secret-immutable"
				}

				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: secretName, Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				switch tt.name {
				case "create tls secret with ca only":
					g.Expect(secret.Data).To(HaveKey("ca.crt"))
				case "create tls secret with cert and key":
					g.Expect(secret.Data).To(HaveKey("tls.crt"))
					g.Expect(secret.Data).To(HaveKey("tls.key"))
				case "create tls secret with all certs":
					g.Expect(secret.Data).To(HaveKey("tls.crt"))
					g.Expect(secret.Data).To(HaveKey("tls.key"))
					g.Expect(secret.Data).To(HaveKey("ca.crt"))
				case "create tls secret with annotations and labels":
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				case "create immutable tls secret":
					g.Expect(secret.Immutable).ToNot(BeNil())
					g.Expect(*secret.Immutable).To(BeTrue())
				}

				defer func() {
					_ = testClient.Delete(ctx, secret)
				}()
			}
		})
	}
}
