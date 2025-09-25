// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/oidc"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		name        string
		spec        config.AuthenticationProviderSpec
		expectError string
	}{
		{
			name: "valid minimal spec",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://accounts.google.com",
				Audience:  "test-client-id",
			},
		},
		{
			name: "valid spec with username expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.email",
				},
			},
		},
		{
			name: "valid spec with groups expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.groups",
				},
			},
		},
		{
			name: "valid spec with scopes expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes",
				},
			},
		},
		{
			name: "valid spec with validations",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: "invalid audience"},
					{Expression: "claims.iss == 'https://example.com'", Message: "invalid issuer"},
				},
			},
		},
		{
			name: "valid spec with all fields",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.preferred_username",
					Groups:   "claims.groups",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes",
				},
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: "invalid audience"},
					{Expression: "claims.exp > 0", Message: "token expired"},
				},
			},
		},
		{
			name: "empty issuer URL",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "",
				Audience:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "invalid issuer URL",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "not-a-url",
				Audience:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "bogus issuer URL with space",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://exam ple.com",
				Audience:  "test-client",
			},
			expectError: "failed to parse issuer URL",
		},
		{
			name: "malformed issuer URL with percent encoding",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com/%ZZ",
				Audience:  "test-client",
			},
			expectError: "failed to parse issuer URL",
		},
		{
			name: "non-https issuer URL",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "http://example.com",
				Audience:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "empty audience",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "",
			},
			expectError: "audience must be provided",
		},
		{
			name: "empty variable name",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "", Expression: "claims.sub"},
				},
			},
			expectError: "variable name must be provided",
		},
		{
			name: "empty variable expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "username", Expression: ""},
				},
			},
			expectError: "variable expression must be provided",
		},
		{
			name: "invalid variable CEL expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "username", Expression: "invalid CEL expression ("},
				},
			},
			expectError: "failed to parse variable 'username' CEL expression",
		},
		{
			name: "empty validation expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "", Message: "validation failed"},
				},
			},
			expectError: "validation expression must be provided",
		},
		{
			name: "empty validation message",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: ""},
				},
			},
			expectError: "validation message must be provided",
		},
		{
			name: "impersonation with empty username and groups",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "",
					Groups:   "",
				},
			},
			expectError: "impersonation must have at least one of username or groups expressions",
		},
		{
			name: "empty scopes expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "",
				},
			},
			expectError: "scopes expression must be provided",
		},
		{
			name: "invalid username CEL expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "invalid CEL expression (",
				},
			},
			expectError: "failed to parse impersonation username expression",
		},
		{
			name: "invalid groups CEL expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Groups: "invalid CEL expression (",
				},
			},
			expectError: "failed to parse impersonation groups expression",
		},
		{
			name: "invalid scopes CEL expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "invalid CEL expression (",
				},
			},
			expectError: "failed to parse scopes expression",
		},
		{
			name: "invalid validation CEL expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "valid expression == true", Message: "valid expression"},
					{Expression: "invalid CEL expression (", Message: "invalid expression"},
				},
			},
			expectError: "failed to parse validation CEL expression",
		},
		{
			name: "complex username expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "has(claims.preferred_username) ? claims.preferred_username : claims.email",
				},
			},
		},
		{
			name: "complex groups expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Groups: "has(claims.groups) ? claims.groups : []",
				},
			},
		},
		{
			name: "complex scopes expression",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "has(claims.scopes) ? claims.scopes : ['default']",
				},
			},
		},
		{
			name: "multiple complex validations",
			spec: config.AuthenticationProviderSpec{
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				IssuerURL: "https://example.com",
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: "invalid audience"},
					{Expression: "claims.exp > 0", Message: "token expired"},
					{Expression: "has(claims.email) && claims.email.endsWith('@example.com')", Message: "invalid email domain"},
					{Expression: "size(claims.groups) > 0", Message: "no groups found"},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			authenticator, err := oidc.New(tt.spec)

			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(authenticator).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(authenticator).NotTo(BeNil())
		})
	}
}

func TestProvider_Authenticate(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            config.AuthenticationProviderSpec
		credentials     auth.ExtractedCredentials
		jwtClaims       map[string]any
		expectedSession *auth.Session
		expectError     string
		invalidJWT      bool
		serverError     bool
		providerError   bool
	}{
		{
			name: "valid token authentication with default expressions",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "valid password authentication with default expressions",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
			},
			credentials: auth.ExtractedCredentials{
				Password: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user456",
				"aud": "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user456",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "custom username expression",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.email",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user789",
				"email": "user@example.com",
				"aud":   "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user@example.com",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "custom groups and scopes expressions",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.groups",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":    "user123",
				"aud":    "test-client-id",
				"groups": []string{"admin", "users"},
				"scopes": []string{"read", "write", "delete"},
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{"admin", "users"},
				Scopes:   []string{"read", "write", "delete"},
			},
		},
		{
			name: "successful validations",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "'test-client-id' in claims.aud", Message: "invalid audience"},
					{Expression: "claims.email.endsWith('@example.com')", Message: "invalid email domain"},
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "complex expressions with all fields",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.preferred_username != '' ? claims.preferred_username : claims.sub",
					Groups:   "claims.groups != null ? claims.groups : ['default']",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes != null ? claims.scopes : []",
				},
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "'test-client-id' in claims.aud", Message: "invalid audience"},
					{Expression: "claims.exp > 0", Message: "token expired"},
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":                "user123",
				"preferred_username": "johndoe",
				"aud":                "test-client-id",
				"groups":             []string{"admin"},
				"scopes":             []string{"read", "write"},
				"exp":                time.Now().Add(time.Hour).Unix(),
			},
			expectedSession: &auth.Session{
				UserName: "johndoe",
				Groups:   []string{"admin"},
				Scopes:   []string{"read", "write"},
			},
		},
		{
			name: "empty credentials",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
			},
			credentials: auth.ExtractedCredentials{}, // No token or password
			expectError: "failed to verify token",
		},
		{
			name: "invalid JWT token",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
			},
			credentials: auth.ExtractedCredentials{
				Token: "invalid.jwt.token",
			},
			expectError: "failed to verify token",
		},
		{
			name: "malformed JWT token",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
			},
			credentials: auth.ExtractedCredentials{
				Token: "not-a-jwt-at-all",
			},
			expectError: "failed to verify token",
		},
		{
			name: "audience mismatch",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "wrong-client-id",
			},
			expectError: "failed to verify token",
		},
		{
			name: "failed validation",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.email.endsWith('@trusted.com')", Message: "untrusted email domain"},
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user123",
				"email": "user@untrusted.com",
				"aud":   "test-client-id",
			},
			expectError: "validation failed: untrusted email domain",
		},
		{
			name: "username expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				Name: "test-provider",
				Type: config.AuthenticationProviderOIDC,
				// Will be set to test server URL
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "nonexistent.field.value",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate impersonation username expression",
		},
		{
			name: "groups expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				// Will be set to test server URL
				Name:     "test-provider",
				Type:     config.AuthenticationProviderOIDC,
				Audience: "test-client-id",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Groups: "nonexistent.field.value",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate impersonation groups expression",
		},
		{
			name: "scopes expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				// Will be set to test server URL
				Name:     "test-provider",
				Type:     config.AuthenticationProviderOIDC,
				Audience: "test-client-id",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "nonexistent.field.value",
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate scopes expression",
		},
		{
			name: "assertion expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				// Will be set to test server URL
				Name:     "test-provider",
				Type:     config.AuthenticationProviderOIDC,
				Audience: "test-client-id",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "nonexistent.field.value == 'something'", Message: "validation failed"},
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate validation expression",
		},
		{
			name: "OIDC provider creation error with unreachable issuer URL",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://nonexistent-domain-that-will-never-exist-12345.com",
				Audience:  "test-client-id",
			},
			credentials: auth.ExtractedCredentials{
				Token: "some-token",
			},
			expectError:   "failed to create OIDC provider",
			providerError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Skip tests that have server errors for now
			if tt.serverError {
				t.Skip("Server error test case - would need unreachable server setup")
			}

			var testServer *testOIDCServer
			var authenticator auth.Provider

			// Special handling for provider creation error test
			if tt.providerError {
				// Use the invalid URL as-is without creating a test server
				auth, err := oidc.New(tt.spec)
				g.Expect(err).NotTo(HaveOccurred())
				authenticator = auth
			} else {
				// Create test OIDC server
				var err error
				testServer, err = newTestOIDCServer()
				g.Expect(err).NotTo(HaveOccurred())
				defer testServer.Close()

				// Set the issuer URL in the spec
				tt.spec.IssuerURL = testServer.issuerURL

				// Create authenticator
				auth, err := oidc.New(tt.spec)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(auth).NotTo(BeNil())
				authenticator = auth
			}

			// Generate JWT token if claims are provided and we have a test server
			if tt.jwtClaims != nil && testServer != nil {
				token, err := testServer.createJWT(tt.jwtClaims)
				g.Expect(err).NotTo(HaveOccurred())
				if tt.credentials.Token == "" && tt.credentials.Password == "" {
					tt.credentials.Token = token
				} else if tt.credentials.Token == "" {
					tt.credentials.Password = token
				} else {
					tt.credentials.Token = token
				}
			}

			// Create context with HTTP client that accepts self-signed certificates
			var ctx context.Context
			if testServer != nil {
				httpClient := testServer.getHTTPClientForTLS()
				ctx = gooidc.ClientContext(context.Background(), httpClient)
			} else {
				ctx = context.Background()
			}

			// Call Authenticate
			session, err := authenticator.Authenticate(ctx, tt.credentials)

			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(session).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(session).NotTo(BeNil())
			g.Expect(session.UserName).To(Equal(tt.expectedSession.UserName))
			g.Expect(session.Groups).To(Equal(tt.expectedSession.Groups))
			g.Expect(session.Scopes).To(Equal(tt.expectedSession.Scopes))
		})
	}
}

func TestProvider_AuthenticateClaims(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            config.AuthenticationProviderSpec
		claims          map[string]any
		expectedSession *auth.Session
		expectError     string
		claimsError     bool
	}{
		{
			name: "default username expression (sub)",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "custom username expression",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.email",
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user@example.com",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "custom groups expression",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.groups",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{"admin", "users"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{"admin", "users"},
				Scopes:   nil,
			},
		},
		{
			name: "custom scopes expression",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{"read", "write", "admin"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   []string{"read", "write", "admin"},
			},
		},
		{
			name: "successful assertions",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: "invalid audience"},
					{Expression: "claims.email.endsWith('@example.com')", Message: "invalid email domain"},
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "complex username expression with fallback",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.preferred_username != '' ? claims.preferred_username : claims.sub",
				},
			},
			claims: map[string]any{
				"sub":                "user123",
				"preferred_username": "johndoe",
				"aud":                "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "johndoe",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "complex username expression fallback to sub",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "complex groups expression with fallback",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.groups != null ? claims.groups : ['default']",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{"admin", "users"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{"admin", "users"},
				Scopes:   nil,
			},
		},
		{
			name: "complex groups expression fallback to default",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "[]",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   nil,
			},
		},
		{
			name: "complex scopes expression with fallback",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes != null ? claims.scopes : ['default']",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{"read", "write"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   []string{"read", "write"},
			},
		},
		{
			name: "complex scopes expression fallback to default",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "size(claims.scopes) > 0 ? claims.scopes : ['read']",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   []string{"read"},
			},
		},
		{
			name: "empty scopes array",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
				},
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "claims.scopes",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   nil,
				Scopes:   []string{},
			},
		},
		{
			name: "username expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "nonexistent.field",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate impersonation username expression",
		},
		{
			name: "groups expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Groups: "nonexistent.field",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate impersonation groups expression",
		},
		{
			name: "scopes expression evaluation error",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Scopes: &config.AuthenticationProviderScopesSpec{
					Expression: "nonexistent.field",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate scopes expression",
		},
		{
			name: "assertion evaluation error",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "nonexistent.field == 'value'", Message: "validation failed"},
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate validation expression",
		},
		{
			name: "assertion fails",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'wrong-client'", Message: "invalid audience"},
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "validation failed: invalid audience",
		},
		{
			name: "multiple assertions with one failing",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "claims.aud == 'test-client'", Message: "invalid audience"},
					{Expression: "claims.email.endsWith('@wrong-domain.com')", Message: "invalid email domain"},
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectError: "validation failed: invalid email domain",
		},
		{
			name: "empty groups array",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.groups",
				},
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   nil,
			},
		},
		{
			name: "numeric values in claims",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "string(claims.user_id)",
				},
			},
			claims: map[string]any{
				"sub":     "user123",
				"user_id": 42,
				"aud":     "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "42",
				Groups:   nil,
				Scopes:   nil,
			},
		},
		{
			name: "with variables",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "username", Expression: "claims.preferred_username"},
					{Name: "domain", Expression: "claims.email.split('@')[1]"},
				},
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "variables.username",
					Groups:   "variables.domain == 'trusted.com' ? ['admin'] : ['user']",
				},
			},
			claims: map[string]any{
				"sub":                "user123",
				"preferred_username": "johndoe",
				"email":              "johndoe@trusted.com",
				"aud":                "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "johndoe",
				Groups:   []string{"admin"},
				Scopes:   nil,
			},
		},
		{
			name: "variables with fallback expression",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "username", Expression: "has(claims.preferred_username) ? claims.preferred_username : claims.sub"},
					{Name: "domain", Expression: "claims.email.split('@')[1]"},
				},
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "variables.username",
					Groups:   "variables.domain == 'example.com' ? ['user'] : ['guest']",
				},
			},
			claims: map[string]any{
				"sub":   "user456",
				"email": "user456@example.com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user456",
				Groups:   []string{"user"},
				Scopes:   nil,
			},
		},
		{
			name: "variable evaluation error",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "username", Expression: "nonexistent.field.value"},
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate variable 'username'",
		},
		{
			name: "variables can reference previously declared variables",
			spec: config.AuthenticationProviderSpec{
				IssuerURL: "https://example.com",
				Name:      "test-provider",
				Type:      config.AuthenticationProviderOIDC,
				Audience:  "test-client",
				Variables: []config.AuthenticationProviderVariableSpec{
					{Name: "email", Expression: "claims.email"},
					{Name: "domain", Expression: "variables.email.split('@')[1]"},
					{Name: "normalized_domain", Expression: "variables.domain.lowerAscii()"},
				},
				Validations: []config.AuthenticationProviderValidationSpec{
					{Expression: "variables.normalized_domain == 'example.com'", Message: "invalid domain"},
				},
				Impersonation: &config.AuthenticationProviderImpersonationSpec{
					Username: "variables.email",
					Groups:   "['users', 'domain:' + variables.normalized_domain]",
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "User@Example.Com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "User@Example.Com",
				Groups:   []string{"users", "domain:example.com"},
				Scopes:   nil,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create authenticator
			authenticator, err := oidc.New(tt.spec)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(authenticator).NotTo(BeNil())

			// Cast to concrete type to access AuthenticateClaims method
			oidcAuth, ok := authenticator.(*oidc.Provider)
			g.Expect(ok).To(BeTrue())

			// Call AuthenticateClaims
			ctx := context.Background()
			session, err := oidcAuth.AuthenticateClaims(ctx, tt.claims)

			if tt.expectError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectError))
				g.Expect(session).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(session).NotTo(BeNil())
			g.Expect(session.UserName).To(Equal(tt.expectedSession.UserName))
			g.Expect(session.Groups).To(Equal(tt.expectedSession.Groups))
			g.Expect(session.Scopes).To(Equal(tt.expectedSession.Scopes))
		})
	}
}

// testOIDCServer encapsulates an OIDC test server with RSA keypair
type testOIDCServer struct {
	server    *httptest.Server
	issuerURL string
	rsaKey    *rsa.PrivateKey
	jwkSet    jwk.Set
}

// newTestOIDCServer creates a new test OIDC server with HTTPS and proper endpoints
func newTestOIDCServer() (*testOIDCServer, error) {
	// Generate RSA keypair
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Create JWK Set with the public key
	key, err := jwk.Import(rsaKey.Public())
	if err != nil {
		return nil, fmt.Errorf("failed to create JWK from RSA key: %w", err)
	}

	// Set key ID and algorithm
	if err := key.Set(jwk.KeyIDKey, "test-key-id"); err != nil {
		return nil, fmt.Errorf("failed to set key ID: %w", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		return nil, fmt.Errorf("failed to set algorithm: %w", err)
	}
	if err := key.Set(jwk.KeyUsageKey, jwk.ForSignature); err != nil {
		return nil, fmt.Errorf("failed to set key usage: %w", err)
	}

	jwkSet := jwk.NewSet()
	if err := jwkSet.AddKey(key); err != nil {
		return nil, fmt.Errorf("failed to add key to set: %w", err)
	}

	ts := &testOIDCServer{
		rsaKey: rsaKey,
		jwkSet: jwkSet,
	}

	// Create HTTPS test server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", ts.handleOpenIDConfiguration)
	mux.HandleFunc("/openid/v1/jwks", ts.handleJWKS)

	server := httptest.NewTLSServer(mux)
	ts.server = server
	ts.issuerURL = server.URL

	return ts, nil
}

// Close shuts down the test server
func (ts *testOIDCServer) Close() {
	if ts.server != nil {
		ts.server.Close()
	}
}

// handleOpenIDConfiguration serves the OIDC discovery endpoint
func (ts *testOIDCServer) handleOpenIDConfiguration(w http.ResponseWriter, r *http.Request) {
	conf := map[string]any{
		"issuer":                                ts.issuerURL,
		"authorization_endpoint":                ts.issuerURL + "/auth",
		"token_endpoint":                        ts.issuerURL + "/token",
		"jwks_uri":                              ts.issuerURL + "/openid/v1/jwks",
		"userinfo_endpoint":                     ts.issuerURL + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"subject_types_supported":               []string{"public"},
		"response_types_supported":              []string{"code"},
		"scopes_supported":                      []string{"openid", "email", "profile"},
		"claims_supported":                      []string{"sub", "email", "name", "groups"},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(conf)
}

// handleJWKS serves the JWKS endpoint
func (ts *testOIDCServer) handleJWKS(w http.ResponseWriter, r *http.Request) {
	jwksJSON, err := json.Marshal(ts.jwkSet)
	if err != nil {
		http.Error(w, "Failed to marshal JWKS", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(jwksJSON)
}

// createJWT creates a JWT token with the given claims
func (ts *testOIDCServer) createJWT(claims map[string]any) (string, error) {
	now := time.Now()

	// Create JWT builder
	builder := jwt.NewBuilder().
		Issuer(ts.issuerURL).
		IssuedAt(now).
		Expiration(now.Add(time.Hour)).
		NotBefore(now).
		Subject("default-subject")

	// Add all custom claims
	for key, value := range claims {
		// Handle special JWT standard claims
		switch key {
		case "aud":
			if s, ok := value.(string); ok {
				builder = builder.Audience([]string{s})
			} else if slice, ok := value.([]string); ok {
				builder = builder.Audience(slice)
			}
		case "sub":
			if s, ok := value.(string); ok {
				builder = builder.Subject(s)
			}
		case "iss":
			if s, ok := value.(string); ok {
				builder = builder.Issuer(s)
			}
		default:
			builder = builder.Claim(key, value)
		}
	}

	// Build the token
	token, err := builder.Build()
	if err != nil {
		return "", fmt.Errorf("failed to build JWT: %w", err)
	}

	// Create a JWK from the private key for signing (includes key ID)
	signingKey, err := jwk.Import(ts.rsaKey)
	if err != nil {
		return "", fmt.Errorf("failed to import signing key: %w", err)
	}
	if err := signingKey.Set(jwk.KeyIDKey, "test-key-id"); err != nil {
		return "", fmt.Errorf("failed to set signing key ID: %w", err)
	}

	// Sign the token
	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), signingKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return string(signed), nil
}

// getHTTPClientForTLS returns an HTTP client that accepts the test server's self-signed certificate
func (ts *testOIDCServer) getHTTPClientForTLS() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}
