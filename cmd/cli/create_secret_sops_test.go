// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"io"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

func TestCreateSecretSOPSCmd(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-sops")
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name         string
		args         []string
		stdin        string
		expectError  bool
		expectExport bool
		expectAgeKey bool
		expectGPGKey bool
	}{
		{
			name:         "create sops secret with age key from stdin",
			args:         []string{"create", "secret", "sops", "test-sops", "--age-key-stdin"},
			stdin:        "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF",
			expectError:  false,
			expectAgeKey: true,
		},
		{
			name:         "create sops secret with age key from stdin and export",
			args:         []string{"create", "secret", "sops", "test-sops", "--age-key-stdin", "--export"},
			stdin:        "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF",
			expectError:  false,
			expectExport: true,
			expectAgeKey: true,
		},
		{
			name:         "create sops secret with gpg key from stdin",
			args:         []string{"create", "secret", "sops", "test-sops", "--gpg-key-stdin"},
			stdin:        "-----BEGIN PGP PRIVATE KEY BLOCK-----\n...test gpg key content...\n-----END PGP PRIVATE KEY BLOCK-----",
			expectError:  false,
			expectGPGKey: true,
		},
		{
			name:         "create sops secret with gpg key from stdin and export",
			args:         []string{"create", "secret", "sops", "test-sops", "--gpg-key-stdin", "--export"},
			stdin:        "-----BEGIN PGP PRIVATE KEY BLOCK-----\n...test gpg key content...\n-----END PGP PRIVATE KEY BLOCK-----",
			expectError:  false,
			expectExport: true,
			expectGPGKey: true,
		},
		{
			name:        "create sops secret with both age and gpg keys from stdin",
			args:        []string{"create", "secret", "sops", "test-sops", "--age-key-stdin", "--gpg-key-stdin"},
			stdin:       "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF\n-----BEGIN PGP PRIVATE KEY BLOCK-----\n...test gpg key content...\n-----END PGP PRIVATE KEY BLOCK-----",
			expectError: true,
		},
		{
			name:         "create sops secret with annotations and labels",
			args:         []string{"create", "secret", "sops", "test-sops", "--age-key-stdin", "--annotation=test.io/annotation=value", "--label=test.io/label=value"},
			stdin:        "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF",
			expectError:  false,
			expectAgeKey: true,
		},
		{
			name:         "create immutable sops secret",
			args:         []string{"create", "secret", "sops", "test-sops", "--age-key-stdin", "--immutable"},
			stdin:        "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF",
			expectError:  false,
			expectAgeKey: true,
		},
		{
			name:        "missing keys",
			args:        []string{"create", "secret", "sops", "test-sops"},
			expectError: true,
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "sops", "--age-key-stdin"},
			stdin:       "AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Set up stdin if provided
			if tt.stdin != "" {
				rootCmd.SetIn(strings.NewReader(tt.stdin))
			} else {
				rootCmd.SetIn(io.NopCloser(strings.NewReader("")))
			}

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
				g.Expect(obj.GetName()).To(Equal("test-sops"))
				g.Expect(obj.GetNamespace()).To(Equal(ns.Name))

				// Verify secret type
				secretType, found, err := unstructured.NestedString(obj.Object, "type")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secretType).To(Equal(string(corev1.SecretTypeOpaque)))

				// Verify stringData contains expected keys
				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())

				if tt.expectAgeKey {
					// Find age key (ends with .agekey)
					ageKeyFound := false
					for key, value := range stringData {
						if strings.HasSuffix(key, ".agekey") {
							ageKeyFound = true
							g.Expect(value).To(ContainSubstring("AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF"))
						}
					}
					g.Expect(ageKeyFound).To(BeTrue())
				}

				if tt.expectGPGKey {
					// Find GPG key (ends with .asc)
					gpgKeyFound := false
					for key, value := range stringData {
						if strings.HasSuffix(key, ".asc") {
							gpgKeyFound = true
							g.Expect(value).To(ContainSubstring("-----BEGIN PGP PRIVATE KEY BLOCK-----"))
							g.Expect(value).To(ContainSubstring("-----END PGP PRIVATE KEY BLOCK-----"))
						}
					}
					g.Expect(gpgKeyFound).To(BeTrue())
				}

				// Verify clean export - unwanted metadata should be removed
				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				// Verify secret was created in cluster
				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: "test-sops", Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify secret type and data
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))

				if tt.expectAgeKey {
					// Find age key (ends with .agekey)
					ageKeyFound := false
					for key, value := range secret.Data {
						if strings.HasSuffix(key, ".agekey") {
							ageKeyFound = true
							g.Expect(string(value)).To(ContainSubstring("AGE-SECRET-KEY-1EXAMPLE123456789ABCDEF"))
						}
					}
					g.Expect(ageKeyFound).To(BeTrue())
				}

				if tt.expectGPGKey {
					// Find GPG key (ends with .asc)
					gpgKeyFound := false
					for key, value := range secret.Data {
						if strings.HasSuffix(key, ".asc") {
							gpgKeyFound = true
							g.Expect(string(value)).To(ContainSubstring("-----BEGIN PGP PRIVATE KEY BLOCK-----"))
							g.Expect(string(value)).To(ContainSubstring("-----END PGP PRIVATE KEY BLOCK-----"))
						}
					}
					g.Expect(gpgKeyFound).To(BeTrue())
				}

				// Test metadata if specified
				if tt.name == "create sops secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				// Test immutable flag
				if tt.name == "create immutable sops secret" {
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
