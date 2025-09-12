// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package factory_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/factory"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		name        string
		config      string
		expectError string
	}{
		{
			name: "valid config with OIDC provider and bearer token credential",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
		},
		{
			name: "valid config with basic auth credential",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BasicAuth
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
		},
		{
			name: "valid config with custom HTTP header credential",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: CustomHTTPHeader
      headers:
        username: X-Username
        password: X-Password
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
		},
		{
			name: "valid config with multiple credentials and providers",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    - type: BasicAuth
    - type: CustomHTTPHeader
      headers:
        token: X-Token
    providers:
    - name: test-oidc-1
      type: OIDC
      issuerurl: https://example1.com
      audience: test-client-1
    - name: test-oidc-2
      type: OIDC
      issuerurl: https://example2.com
      audience: test-client-2
`,
		},
		{
			name: "valid config with OIDC optional fields",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
      impersonation:
        username: claims.username
        groups: claims.groups
      validations:
      - expression: claims.aud == "test-client"
        message: invalid audience
`,
		},
		{
			name:        "empty config",
			config:      "",
			expectError: "no credentials found in the authentication configuration",
		},
		{
			name:        "invalid YAML",
			config:      "invalid: yaml: content:",
			expectError: "yaml: mapping values are not allowed in this context",
		},
		{
			name: "unsupported credential type",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: UnsupportedCredential
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "unsupported credential type 'UnsupportedCredential' in the authentication configuration",
		},
		{
			name: "missing headers for custom HTTP header credential",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: CustomHTTPHeader
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "missing headers configuration for CustomHTTPHeader credential at index 0",
		},
		{
			name: "no credentials",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials: []
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "no credentials found in the authentication configuration",
		},
		{
			name: "missing type in provider reference",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "missing type in provider reference at index 0",
		},
		{
			name: "missing name in provider reference",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "missing name in provider reference at index 0",
		},
		{
			name: "duplicate provider name",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "duplicate provider name 'test-oidc' in the authentication configuration",
		},
		{
			name: "unknown provider type",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-auth
      type: UnsupportedProvider
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "unknown provider type 'UnsupportedProvider' in the authentication configuration",
		},
		{
			name: "no providers",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers: []
`,
			expectError: "no providers found in the authentication configuration",
		},
		{
			name: "valid OIDC provider with minimal spec",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
		},
		{
			name: "OIDC provider with invalid issuer URL",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    providers:
    - name: test-oidc
      type: OIDC
      issuerURL: https://exa mple.com
      audience: test-client
`,
			expectError: "failed to create OIDC provider test-oidc",
		},
		{
			name: "credential with missing headers at different index",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: Config
spec:
  authentication:
    credentials:
    - type: BearerToken
    - type: BasicAuth
    - type: CustomHTTPHeader
    providers:
    - name: test-oidc
      type: OIDC
      issuerurl: https://example.com
      audience: test-client
`,
			expectError: "missing headers configuration for CustomHTTPHeader credential at index 2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var conf config.Config
			err := yaml.Unmarshal([]byte(tt.config), &conf)

			if tt.expectError != "" {
				if err != nil {
					g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
					return
				}
				g.Expect(err).NotTo(HaveOccurred())
				var authSpec config.AuthenticationSpec
				if conf.Spec.Authentication != nil {
					authSpec = *conf.Spec.Authentication
				}
				middleware, factoryErr := factory.New(authSpec)
				g.Expect(factoryErr).To(HaveOccurred())
				g.Expect(factoryErr.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(middleware).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(conf.Spec.Authentication).NotTo(BeNil())
			middleware, err := factory.New(*conf.Spec.Authentication)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(middleware).NotTo(BeNil())
		})
	}
}
