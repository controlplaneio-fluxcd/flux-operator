// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

func TestCreateSecretRegistryCmd(t *testing.T) {
	g := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "test-registry")
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name         string
		args         []string
		expectError  bool
		expectExport bool
	}{
		{
			name:        "create registry secret",
			args:        []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--username=admin", "--password=secret"},
			expectError: false,
		},
		{
			name:         "create registry secret with export",
			args:         []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--username=admin", "--password=secret", "--export"},
			expectError:  false,
			expectExport: true,
		},
		{
			name:        "create registry secret with annotations and labels",
			args:        []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--username=admin", "--password=secret", "--annotation=test.io/annotation=value", "--label=test.io/label=value"},
			expectError: false,
		},
		{
			name:        "create immutable registry secret",
			args:        []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--username=admin", "--password=secret", "--immutable"},
			expectError: false,
		},
		{
			name:         "missing server",
			args:         []string{"create", "secret", "registry", "test-registry", "--username=admin", "--password=secret", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:         "missing username",
			args:         []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--password=secret", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:         "missing password",
			args:         []string{"create", "secret", "registry", "test-registry", "--server=ghcr.io", "--username=admin", "--export"},
			expectError:  true,
			expectExport: true,
		},
		{
			name:        "missing secret name",
			args:        []string{"create", "secret", "registry", "--server=ghcr.io", "--username=admin", "--password=secret"},
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
				g.Expect(obj.GetName()).To(Equal("test-registry"))
				g.Expect(obj.GetNamespace()).To(Equal(ns.Name))

				// Verify secret type
				secretType, found, err := unstructured.NestedString(obj.Object, "type")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secretType).To(Equal(string(corev1.SecretTypeDockerConfigJson)))

				// Verify stringData contains .dockerconfigjson
				stringData, found, err := unstructured.NestedStringMap(obj.Object, "stringData")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(stringData).To(HaveKey(".dockerconfigjson"))

				// Verify the Docker config JSON structure
				dockerConfigJSON := stringData[".dockerconfigjson"]
				var dockerConfig map[string]any
				err = json.Unmarshal([]byte(dockerConfigJSON), &dockerConfig)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(dockerConfig).To(HaveKey("auths"))

				auths := dockerConfig["auths"].(map[string]any)
				g.Expect(auths).To(HaveKey("ghcr.io"))

				ghcrAuth := auths["ghcr.io"].(map[string]any)
				g.Expect(ghcrAuth).To(HaveKey("username"))
				g.Expect(ghcrAuth).To(HaveKey("password"))
				g.Expect(ghcrAuth).To(HaveKey("auth"))
				g.Expect(ghcrAuth["username"]).To(Equal("admin"))
				g.Expect(ghcrAuth["password"]).To(Equal("secret"))

				// Verify clean export - unwanted metadata should be removed
				_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(found).To(BeFalse())
			} else {
				// Verify secret was created in cluster
				secret := &corev1.Secret{}
				secretKey := types.NamespacedName{Name: "test-registry", Namespace: ns.Name}
				err = testClient.Get(ctx, secretKey, secret)
				g.Expect(err).ToNot(HaveOccurred())

				// Verify secret type and data
				g.Expect(secret.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				g.Expect(secret.Data).To(HaveKey(".dockerconfigjson"))

				// Verify the Docker config JSON structure
				dockerConfigJSON := string(secret.Data[".dockerconfigjson"])
				var dockerConfig map[string]any
				err = json.Unmarshal([]byte(dockerConfigJSON), &dockerConfig)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(dockerConfig).To(HaveKey("auths"))

				auths := dockerConfig["auths"].(map[string]any)
				g.Expect(auths).To(HaveKey("ghcr.io"))

				ghcrAuth := auths["ghcr.io"].(map[string]any)
				g.Expect(ghcrAuth).To(HaveKey("username"))
				g.Expect(ghcrAuth).To(HaveKey("password"))
				g.Expect(ghcrAuth).To(HaveKey("auth"))
				g.Expect(ghcrAuth["username"]).To(Equal("admin"))
				g.Expect(ghcrAuth["password"]).To(Equal("secret"))

				// Test metadata if specified
				if tt.name == "create registry secret with annotations and labels" {
					g.Expect(secret.Annotations).To(HaveKeyWithValue("test.io/annotation", "value"))
					g.Expect(secret.Labels).To(HaveKeyWithValue("test.io/label", "value"))
				}

				// Test immutable flag
				if tt.name == "create immutable registry secret" {
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
