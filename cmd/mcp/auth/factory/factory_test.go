// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package factory_test

import (
	"io"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/factory"
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		name        string
		config      string
		expectError string
	}{
		{
			name: "valid config with OIDC authenticator and bearer token transport",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
		},
		{
			name: "valid config with basic auth transport",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BasicAuth
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
		},
		{
			name: "valid config with custom HTTP header transport",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: CustomHTTPHeader
    headers:
      username: X-Username
      password: X-Password
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
		},
		{
			name: "valid config with multiple transports and authenticators",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  - type: BasicAuth
  - type: CustomHTTPHeader
    headers:
      token: X-Token
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc-1
  - kind: OIDCAuthenticator
    name: test-oidc-2
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc-1
spec:
  issuerURL: https://example1.com
  clientID: test-client-1
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc-2
spec:
  issuerURL: https://example2.com
  clientID: test-client-2
`,
		},
		{
			name: "valid config with OIDC optional fields",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
  username: claims.username
  groups: claims.groups
  assertions:
  - claims.aud == "test-client"
`,
		},
		{
			name:        "empty config",
			config:      "",
			expectError: "no authentication configuration object found in the auth config file",
		},
		{
			name:        "invalid YAML",
			config:      "invalid: yaml: content:",
			expectError: "failed to unmarshal auth config object from YAML",
		},
		{
			name: "unsupported API version",
			config: `apiVersion: v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators: []
`,
			expectError: "unsupported apiVersion 'v1' in the auth config file",
		},
		{
			name: "missing metadata name",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata: {}
spec:
  transports:
  - type: BearerToken
  authenticators: []
`,
			expectError: "missing metadata.name in the auth config file",
		},
		{
			name: "unsupported kind",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: UnsupportedKind
metadata:
  name: test-config
spec: {}
`,
			expectError: "unsupported kind 'UnsupportedKind' in the auth config file",
		},
		{
			name: "multiple authentication configurations",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config-1
spec:
  transports:
  - type: BearerToken
  authenticators: []
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config-2
spec:
  transports:
  - type: BearerToken
  authenticators: []
`,
			expectError: "multiple authentication configuration objects found in the auth config file",
		},
		{
			name: "unsupported transport type",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: UnsupportedTransport
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "unsupported transport type 'UnsupportedTransport' in the auth config file",
		},
		{
			name: "missing headers for custom HTTP header transport",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: CustomHTTPHeader
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "missing headers configuration for CustomHTTPHeader transport at index 0",
		},
		{
			name: "no transports",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports: []
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "no transports found in the authentication configuration",
		},
		{
			name: "missing kind in authenticator reference",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "missing kind in authenticator reference at index 0",
		},
		{
			name: "missing name in authenticator reference",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "missing name in authenticator reference at index 0",
		},
		{
			name: "authenticator not found",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: missing-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "authenticator missing-oidc of kind OIDCAuthenticator not found in the auth config file",
		},
		{
			name: "unsupported authenticator kind",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: UnsupportedAuthenticator
    name: test-auth
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: UnsupportedAuthenticator
metadata:
  name: test-auth
spec: {}
`,
			expectError: "unsupported kind 'UnsupportedAuthenticator' in the auth config file",
		},
		{
			name: "no authenticators",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators: []
`,
			expectError: "no authenticators found in the authentication configuration",
		},
		{
			name: "valid OIDC authenticator with empty spec",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
		},
		{
			name: "OIDC authenticator with invalid issuer URL",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://exa mple.com
  clientID: test-client
`,
			expectError: "failed to parse issuer URL",
		},
		{
			name: "malformed OIDC authenticator YAML",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec: invalid
`,
			expectError: "failed to unmarshal OIDC authenticator object from YAML",
		},
		{
			name: "io.EOF handling",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client`,
		},
		{
			name: "transport with missing headers at different index",
			config: `apiVersion: mcp.fluxcd.controlplane.io/v1
kind: AuthenticationConfiguration
metadata:
  name: test-config
spec:
  transports:
  - type: BearerToken
  - type: BasicAuth
  - type: CustomHTTPHeader
  authenticators:
  - kind: OIDCAuthenticator
    name: test-oidc
---
apiVersion: mcp.fluxcd.controlplane.io/v1
kind: OIDCAuthenticator
metadata:
  name: test-oidc
spec:
  issuerURL: https://example.com
  clientID: test-client
`,
			expectError: "missing headers configuration for CustomHTTPHeader transport at index 2",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			reader := strings.NewReader(tt.config)
			middleware, err := factory.New(reader)

			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(middleware).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(middleware).NotTo(BeNil())
		})
	}
}

func TestNew_ReadError(t *testing.T) {
	g := NewWithT(t)

	// Test with a reader that always returns an error (not EOF)
	errorReader := &errorReader{err: io.ErrUnexpectedEOF}
	middleware, err := factory.New(errorReader)

	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to read auth config file"))
	g.Expect(middleware).To(BeNil())
}

// errorReader is a test helper that always returns the specified error when Read is called
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
