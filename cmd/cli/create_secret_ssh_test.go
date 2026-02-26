// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

func TestCreateSecretSSHCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-ssh")
	gt.Expect(err).ToNot(HaveOccurred())

	// Create temporary key and known_hosts files.
	tmpDir := t.TempDir()
	privateKeyFile := filepath.Join(tmpDir, "id_rsa")
	gt.Expect(os.WriteFile(privateKeyFile, []byte("test-private-key"), 0600)).To(Succeed())
	publicKeyFile := filepath.Join(tmpDir, "id_rsa.pub")
	gt.Expect(os.WriteFile(publicKeyFile, []byte("test-public-key"), 0600)).To(Succeed())
	knownHostsFile := filepath.Join(tmpDir, "known_hosts")
	gt.Expect(os.WriteFile(knownHostsFile, []byte("github.com ssh-rsa AAAA..."), 0600)).To(Succeed())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name: "create ssh secret",
			args: []string{
				"create", "secret", "ssh", "test-secret",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=" + knownHostsFile,
			},
		},
		{
			name: "create ssh secret with export",
			args: []string{
				"create", "secret", "ssh", "test-secret",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=" + knownHostsFile,
				"--export",
			},
			expectExport: true,
		},
		{
			name: "create ssh secret with public key",
			args: []string{
				"create", "secret", "ssh", "test-secret-pubkey",
				"--private-key-file=" + privateKeyFile,
				"--public-key-file=" + publicKeyFile,
				"--knownhosts-file=" + knownHostsFile,
			},
		},
		{
			name: "create ssh secret with password",
			args: []string{
				"create", "secret", "ssh", "test-secret-password",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=" + knownHostsFile,
				"--password=mysecret",
			},
		},
		{
			name: "create ssh secret with annotations and labels",
			args: []string{
				"create", "secret", "ssh", "test-secret-meta",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=" + knownHostsFile,
				"--annotation=test.io/annotation=value",
				"--label=test.io/label=value",
			},
		},
		{
			name: "create immutable ssh secret",
			args: []string{
				"create", "secret", "ssh", "test-secret-immutable",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=" + knownHostsFile,
				"--immutable",
			},
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "ssh", "--private-key-file=" + privateKeyFile, "--knownhosts-file=" + knownHostsFile},
			expectError: true,
		},
		{
			name: "missing private key file",
			args: []string{
				"create", "secret", "ssh", "test-secret",
				"--private-key-file=/nonexistent/id_rsa",
				"--knownhosts-file=" + knownHostsFile,
				"--export",
			},
			expectError:  true,
			expectExport: true,
		},
		{
			name: "missing known hosts file",
			args: []string{
				"create", "secret", "ssh", "test-secret",
				"--private-key-file=" + privateKeyFile,
				"--knownhosts-file=/nonexistent/known_hosts",
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

				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(stringData).To(HaveKey("identity"))
				g.Expect(stringData).To(HaveKey("known_hosts"))

				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				secretName := "test-secret"
				switch tt.name {
				case "create ssh secret with public key":
					secretName = "test-secret-pubkey"
				case "create ssh secret with password":
					secretName = "test-secret-password"
				case "create ssh secret with annotations and labels":
					secretName = "test-secret-meta"
				case "create immutable ssh secret":
					secretName = "test-secret-immutable"
				}

				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: secretName, Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(secret.Data).To(HaveKey("identity"))
				g.Expect(secret.Data).To(HaveKey("known_hosts"))
				g.Expect(string(secret.Data["identity"])).To(Equal("test-private-key"))
				g.Expect(string(secret.Data["known_hosts"])).To(Equal("github.com ssh-rsa AAAA..."))

				if tt.name == "create ssh secret with public key" {
					g.Expect(secret.Data).To(HaveKey("identity.pub"))
					g.Expect(string(secret.Data["identity.pub"])).To(Equal("test-public-key"))
				}

				if tt.name == "create ssh secret with password" {
					g.Expect(secret.Data).To(HaveKey("password"))
					g.Expect(string(secret.Data["password"])).To(Equal("mysecret"))
				}

				if tt.name == "create ssh secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				if tt.name == "create immutable ssh secret" {
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
