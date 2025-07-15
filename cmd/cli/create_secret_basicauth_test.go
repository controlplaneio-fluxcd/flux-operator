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

func TestCreateSecretBasicAuthCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-basic-auth")
	gt.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name:        "create basic auth secret",
			args:        []string{"create", "secret", "basic-auth", "test-secret", "--username=admin", "--password=secret", "-n", ns.Name},
			expectError: false,
		},
		{
			name:         "create basic auth secret with export",
			args:         []string{"create", "secret", "basic-auth", "test-secret", "--username=admin", "--password=secret", "--export", "-n", ns.Name},
			expectError:  false,
			expectExport: true,
		},
		{
			name:        "create basic auth secret with annotations and labels",
			args:        []string{"create", "secret", "basic-auth", "test-secret", "--username=admin", "--password=secret", "--annotation=test.io/annotation=value", "--label=test.io/label=value", "-n", ns.Name},
			expectError: false,
		},
		{
			name:        "create immutable basic auth secret",
			args:        []string{"create", "secret", "basic-auth", "test-secret", "--username=admin", "--password=secret", "--immutable", "-n", ns.Name},
			expectError: false,
		},
		{
			name:         "missing username",
			args:         []string{"create", "secret", "basic-auth", "test-secret", "--password=secret", "--export", "-n", ns.Name},
			expectError:  true,
			expectExport: true,
		},
		{
			name:         "missing password",
			args:         []string{"create", "secret", "basic-auth", "test-secret", "--username=admin", "--export", "-n", ns.Name},
			expectError:  true,
			expectExport: true,
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "basic-auth", "--username=admin", "--password=secret", "-n", ns.Name},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Execute command
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
				g.Expect(secretType).To(Equal(string(corev1.SecretTypeBasicAuth)))

				// Verify stringData contains username and password
				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(stringData).To(HaveKey("username"))
				g.Expect(stringData).To(HaveKey("password"))

				// Verify clean export - unwanted metadata should be removed
				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				// Verify secret was created in cluster
				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: "test-secret", Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify secret type and data
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeBasicAuth))
				g.Expect(secret.Data).To(HaveKey("username"))
				g.Expect(secret.Data).To(HaveKey("password"))
				g.Expect(string(secret.Data["username"])).To(Equal("admin"))
				g.Expect(string(secret.Data["password"])).To(Equal("secret"))

				// Test metadata if specified
				if tt.name == "create basic auth secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				// Test immutable flag
				if tt.name == "create immutable basic auth secret" {
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
