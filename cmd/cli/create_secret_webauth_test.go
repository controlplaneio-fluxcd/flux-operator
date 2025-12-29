// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

func TestCreateSecretWebAuthCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-web-auth")
	gt.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name:        "create web-auth secret with client-secret",
			args:        []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret=test-secret-value"},
			expectError: false,
		},
		{
			name:        "create web-auth secret with random client-secret",
			args:        []string{"create", "secret", "web-auth", "test-secret-rnd", "--client-id=test-client", "--client-secret-rnd"},
			expectError: false,
		},
		{
			name:         "create web-auth secret with export",
			args:         []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret=test-secret-value", "--export"},
			expectError:  false,
			expectExport: true,
		},
		{
			name:         "create web-auth secret with random and export",
			args:         []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret-rnd", "--export"},
			expectError:  false,
			expectExport: true,
		},
		{
			name:        "create web-auth secret with annotations and labels",
			args:        []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret=test-secret-value", "--annotation=test.io/annotation=value", "--label=test.io/label=value"},
			expectError: false,
		},
		{
			name:        "create immutable web-auth secret",
			args:        []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret=test-secret-value", "--immutable"},
			expectError: false,
		},
		{
			name:         "missing client-id",
			args:         []string{"create", "secret", "web-auth", "test-secret", "--client-secret=test-secret-value", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:         "missing client-secret source",
			args:         []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:         "multiple client-secret sources",
			args:         []string{"create", "secret", "web-auth", "test-secret", "--client-id=test-client", "--client-secret=test", "--client-secret-rnd", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "web-auth", "--client-id=test-client", "--client-secret=test-secret-value"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Execute command
			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			if tt.expectExport {
				// Parse output as unstructured object
				obj := &unstructured.Unstructured{}
				err = yaml.Unmarshal([]byte(output), &obj.Object)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify basic structure
				g.Expect(obj.GetAPIVersion()).To(Equal("v1"))
				g.Expect(obj.GetKind()).To(Equal("Secret"))
				g.Expect(obj.GetName()).To(Equal("test-secret"))
				g.Expect(obj.GetNamespace()).To(Equal(ns.Name))

				// Verify secret type
				secretType, found, err := unstructured.NestedString(obj.Object, "type")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secretType).To(Equal(string(corev1.SecretTypeOpaque)))

				// Verify stringData contains client-id and client-secret
				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(stringData).To(HaveKey("client-id"))
				g.Expect(stringData).To(HaveKey("client-secret"))

				// Verify client-id value
				g.Expect(stringData["client-id"]).To(Equal("test-client"))

				// For random secret, just verify it's not empty
				if tt.name == "create web-auth secret with random and export" {
					g.Expect(stringData["client-secret"]).ToNot(BeEmpty())
					g.Expect(len(stringData["client-secret"])).To(BeNumerically(">=", 32))
				}

				// Verify clean export - unwanted metadata should be removed
				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				// Verify secret was created in cluster
				secretName := "test-secret"
				if tt.name == "create web-auth secret with random client-secret" {
					secretName = "test-secret-rnd"
				}
				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: secretName, Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify secret type and data
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
				g.Expect(secret.Data).To(HaveKey("client-id"))
				g.Expect(secret.Data).To(HaveKey("client-secret"))
				g.Expect(string(secret.Data["client-id"])).To(Equal("test-client"))

				// For non-random secrets, verify the exact value
				if tt.name == "create web-auth secret with client-secret" {
					g.Expect(string(secret.Data["client-secret"])).To(Equal("test-secret-value"))
				}

				// For random secrets, verify it's not empty and has reasonable length
				if tt.name == "create web-auth secret with random client-secret" {
					g.Expect(string(secret.Data["client-secret"])).ToNot(BeEmpty())
					g.Expect(len(secret.Data["client-secret"])).To(BeNumerically(">=", 32))
				}

				// Test metadata if specified
				if tt.name == "create web-auth secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				// Test immutable flag
				if tt.name == "create immutable web-auth secret" {
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
