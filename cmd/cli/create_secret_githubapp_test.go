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

func TestCreateSecretGitHubAppCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-githubapp")
	gt.Expect(err).ToNot(HaveOccurred())

	// Create a temporary private key file for testing.
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "private-key.pem")
	gt.Expect(os.WriteFile(keyFile, []byte("test-private-key-content"), 0600)).To(Succeed())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name: "create github app secret",
			args: []string{
				"create", "secret", "githubapp", "test-secret",
				"--app-id=123456",
				"--app-installation-id=78901234",
				"--app-private-key-file=" + keyFile,
			},
		},
		{
			name: "create github app secret with export",
			args: []string{
				"create", "secret", "githubapp", "test-secret",
				"--app-id=123456",
				"--app-installation-id=78901234",
				"--app-private-key-file=" + keyFile,
				"--export",
			},
			expectExport: true,
		},
		{
			name: "create github app secret with installation owner",
			args: []string{
				"create", "secret", "githubapp", "test-secret-owner",
				"--app-id=123456",
				"--app-installation-owner=my-org",
				"--app-private-key-file=" + keyFile,
				"--app-base-url=https://github.example.com/api/v3",
			},
		},
		{
			name: "create github app secret with annotations and labels",
			args: []string{
				"create", "secret", "githubapp", "test-secret-meta",
				"--app-id=123456",
				"--app-installation-id=78901234",
				"--app-private-key-file=" + keyFile,
				"--annotation=test.io/annotation=value",
				"--label=test.io/label=value",
			},
		},
		{
			name: "create immutable github app secret",
			args: []string{
				"create", "secret", "githubapp", "test-secret-immutable",
				"--app-id=123456",
				"--app-installation-id=78901234",
				"--app-private-key-file=" + keyFile,
				"--immutable",
			},
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "githubapp", "--app-id=123456", "--app-private-key-file=" + keyFile},
			expectError: true,
		},
		{
			name: "missing private key file",
			args: []string{
				"create", "secret", "githubapp", "test-secret",
				"--app-id=123456",
				"--app-private-key-file=/nonexistent/path/key.pem",
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
				// Parse output as unstructured object.
				obj := &unstructured.Unstructured{}
				err = yaml.Unmarshal([]byte(output), &obj.Object)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify basic structure.
				g.Expect(obj.GetAPIVersion()).To(Equal("v1"))
				g.Expect(obj.GetKind()).To(Equal("Secret"))
				g.Expect(obj.GetName()).To(Equal("test-secret"))
				g.Expect(obj.GetNamespace()).To(Equal(ns.Name))

				// Verify secret type.
				secretType, found, err := unstructured.NestedString(obj.Object, "type")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secretType).To(Equal(string(corev1.SecretTypeOpaque)))

				// Verify stringData contains expected keys.
				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(stringData).To(HaveKey("githubAppID"))
				g.Expect(stringData).To(HaveKey("githubAppPrivateKey"))

				// Verify clean export.
				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				// Verify secret was created in cluster.
				secretName := "test-secret"
				switch tt.name {
				case "create github app secret with installation owner":
					secretName = "test-secret-owner"
				case "create github app secret with annotations and labels":
					secretName = "test-secret-meta"
				case "create immutable github app secret":
					secretName = "test-secret-immutable"
				}

				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: secretName, Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify secret type and data.
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.Data).To(HaveKey("githubAppID"))
				g.Expect(secret.Data).To(HaveKey("githubAppPrivateKey"))
				g.Expect(string(secret.Data["githubAppID"])).To(Equal("123456"))

				// Test installation owner and base URL.
				if tt.name == "create github app secret with installation owner" {
					g.Expect(secret.Data).To(HaveKey("githubAppInstallationOwner"))
					g.Expect(string(secret.Data["githubAppInstallationOwner"])).To(Equal("my-org"))
					g.Expect(secret.Data).To(HaveKey("githubAppBaseURL"))
					g.Expect(string(secret.Data["githubAppBaseURL"])).To(Equal("https://github.example.com/api/v3"))
				}

				// Test metadata.
				if tt.name == "create github app secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				// Test immutable flag.
				if tt.name == "create immutable github app secret" {
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
