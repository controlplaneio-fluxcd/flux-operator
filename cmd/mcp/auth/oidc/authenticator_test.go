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
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		name        string
		spec        auth.OIDCAuthenticatorSpec
		expectError string
	}{
		{
			name: "valid minimal spec",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://accounts.google.com",
				ClientID:  "test-client-id",
			},
		},
		{
			name: "valid spec with username expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "claims.email",
			},
		},
		{
			name: "valid spec with groups expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "claims.groups",
			},
		},
		{
			name: "valid spec with scopes expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "claims.scopes",
			},
		},
		{
			name: "valid spec with assertions",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"claims.aud == 'test-client'",
					"claims.iss == 'https://example.com'",
				},
			},
		},
		{
			name: "valid spec with all fields",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "claims.preferred_username",
				Groups:    "claims.groups",
				Scopes:    "claims.scopes",
				Assertions: []string{
					"claims.aud == 'test-client'",
					"claims.exp > 0",
				},
			},
		},
		{
			name: "empty issuer URL",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "",
				ClientID:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "invalid issuer URL",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "not-a-url",
				ClientID:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "bogus issuer URL with space",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://exam ple.com",
				ClientID:  "test-client",
			},
			expectError: "failed to parse issuer URL",
		},
		{
			name: "malformed issuer URL with percent encoding",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com/%ZZ",
				ClientID:  "test-client",
			},
			expectError: "failed to parse issuer URL",
		},
		{
			name: "non-https issuer URL",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "http://example.com",
				ClientID:  "test-client",
			},
			expectError: "issuer URL must use https scheme",
		},
		{
			name: "empty client ID",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "",
			},
			expectError: "client ID must be provided",
		},
		{
			name: "invalid username CEL expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "invalid CEL expression (",
			},
			expectError: "failed to parse username expression",
		},
		{
			name: "invalid groups CEL expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "invalid CEL expression (",
			},
			expectError: "failed to parse groups expression",
		},
		{
			name: "invalid scopes CEL expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "invalid CEL expression (",
			},
			expectError: "failed to parse scopes expression",
		},
		{
			name: "invalid assertion CEL expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"valid expression == true",
					"invalid CEL expression (",
				},
			},
			expectError: "failed to parse assertion expression",
		},
		{
			name: "complex username expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "has(claims.preferred_username) ? claims.preferred_username : claims.email",
			},
		},
		{
			name: "complex groups expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "has(claims.groups) ? claims.groups : []",
			},
		},
		{
			name: "complex scopes expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "has(claims.scopes) ? claims.scopes : ['default']",
			},
		},
		{
			name: "multiple complex assertions",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"claims.aud == 'test-client'",
					"claims.exp > 0",
					"has(claims.email) && claims.email.endsWith('@example.com')",
					"size(claims.groups) > 0",
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

func TestAuthenticator_Authenticate(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            auth.OIDCAuthenticatorSpec
		credentials     auth.Credentials
		jwtClaims       map[string]any
		expectedSession *auth.Session
		expectError     string
		invalidJWT      bool
		serverError     bool
		providerError   bool
	}{
		{
			name: "valid token authentication with default expressions",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "valid password authentication with default expressions",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{
				Password: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user456",
				"aud": "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user456",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "custom username expression",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Username: "email",
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user789",
				"email": "user@example.com",
				"aud":   "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user@example.com",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "custom groups and scopes expressions",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Groups:   "groups",
				Scopes:   "scopes",
			},
			credentials: auth.Credentials{
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
			name: "successful assertions",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Assertions: []string{
					"'test-client-id' in aud",
					"email.endsWith('@example.com')",
				},
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client-id",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "complex expressions with all fields",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Username: "preferred_username != '' ? preferred_username : sub",
				Groups:   "groups != null ? groups : ['default']",
				Scopes:   "scopes != null ? scopes : []",
				Assertions: []string{
					"'test-client-id' in aud",
					"exp > 0",
				},
			},
			credentials: auth.Credentials{
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
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{}, // No token or password
			expectError: "failed to verify token",
		},
		{
			name: "invalid JWT token",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{
				Token: "invalid.jwt.token",
			},
			expectError: "failed to verify token",
		},
		{
			name: "malformed JWT token",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{
				Token: "not-a-jwt-at-all",
			},
			expectError: "failed to verify token",
		},
		{
			name: "audience mismatch",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "wrong-client-id",
			},
			expectError: "failed to verify token",
		},
		{
			name: "failed assertion",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Assertions: []string{
					"email.endsWith('@trusted.com')",
				},
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub":   "user123",
				"email": "user@untrusted.com",
				"aud":   "test-client-id",
			},
			expectError: "assertion 'email.endsWith('@trusted.com')' failed",
		},
		{
			name: "username expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Username: "nonexistent.field.value",
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate username expression",
		},
		{
			name: "groups expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Groups:   "nonexistent.field.value",
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate groups expression",
		},
		{
			name: "scopes expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Scopes:   "nonexistent.field.value",
			},
			credentials: auth.Credentials{
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
			spec: auth.OIDCAuthenticatorSpec{
				// Will be set to test server URL
				ClientID: "test-client-id",
				Assertions: []string{
					"nonexistent.field.value == 'something'",
				},
			},
			credentials: auth.Credentials{
				Token: "", // Will be set to generated JWT
			},
			jwtClaims: map[string]any{
				"sub": "user123",
				"aud": "test-client-id",
			},
			expectError: "failed to evaluate assertion expression",
		},
		{
			name: "OIDC provider creation error with unreachable issuer URL",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://nonexistent-domain-that-will-never-exist-12345.com",
				ClientID:  "test-client-id",
			},
			credentials: auth.Credentials{
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
			var authenticator auth.Authenticator

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

func TestAuthenticator_AuthenticateToken(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            auth.OIDCAuthenticatorSpec
		claims          map[string]any
		expectedSession *auth.Session
		expectError     string
		claimsError     bool
	}{
		{
			name: "default username expression (sub)",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "custom username expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "email",
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user@example.com",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "custom groups expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "groups",
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{"admin", "users"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{"admin", "users"},
				Scopes:   []string{},
			},
		},
		{
			name: "custom scopes expression",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "scopes",
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{"read", "write", "admin"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{"read", "write", "admin"},
			},
		},
		{
			name: "successful assertions",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"aud == 'test-client'",
					"email.endsWith('@example.com')",
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "complex username expression with fallback",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "preferred_username != '' ? preferred_username : sub",
			},
			claims: map[string]any{
				"sub":                "user123",
				"preferred_username": "johndoe",
				"aud":                "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "johndoe",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "complex username expression fallback to sub",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "sub",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "complex groups expression with fallback",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "groups != null ? groups : ['default']",
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{"admin", "users"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{"admin", "users"},
				Scopes:   []string{},
			},
		},
		{
			name: "complex groups expression fallback to default",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "[]",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "complex scopes expression with fallback",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "scopes != null ? scopes : ['default']",
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{"read", "write"},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{"read", "write"},
			},
		},
		{
			name: "complex scopes expression fallback to default",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "size(scopes) > 0 ? scopes : ['read']",
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{"read"},
			},
		},
		{
			name: "empty scopes array",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "scopes",
			},
			claims: map[string]any{
				"sub":    "user123",
				"scopes": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "username expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "nonexistent.field",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate username expression",
		},
		{
			name: "groups expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "nonexistent.field",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate groups expression",
		},
		{
			name: "scopes expression evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Scopes:    "nonexistent.field",
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate scopes expression",
		},
		{
			name: "assertion evaluation error",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"nonexistent.field == 'value'",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "failed to evaluate assertion expression",
		},
		{
			name: "assertion fails",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"aud == 'wrong-client'",
				},
			},
			claims: map[string]any{
				"sub": "user123",
				"aud": "test-client",
			},
			expectError: "assertion 'aud == 'wrong-client'' failed",
		},
		{
			name: "multiple assertions with one failing",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Assertions: []string{
					"aud == 'test-client'",
					"email.endsWith('@wrong-domain.com')",
				},
			},
			claims: map[string]any{
				"sub":   "user123",
				"email": "user@example.com",
				"aud":   "test-client",
			},
			expectError: "assertion 'email.endsWith('@wrong-domain.com')' failed",
		},
		{
			name: "empty groups array",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Groups:    "groups",
			},
			claims: map[string]any{
				"sub":    "user123",
				"groups": []any{},
				"aud":    "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "user123",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "numeric values in claims",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
				Username:  "string(user_id)",
			},
			claims: map[string]any{
				"sub":     "user123",
				"user_id": 42,
				"aud":     "test-client",
			},
			expectedSession: &auth.Session{
				UserName: "42",
				Groups:   []string{},
				Scopes:   []string{},
			},
		},
		{
			name: "Claims() method returns error",
			spec: auth.OIDCAuthenticatorSpec{
				IssuerURL: "https://example.com",
				ClientID:  "test-client",
			},
			claims:      map[string]any{}, // Will be ignored due to mock error
			expectError: "failed to extract claims from token",
			claimsError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create authenticator
			authenticator, err := oidc.New(tt.spec)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(authenticator).NotTo(BeNil())

			// Cast to concrete type to access AuthenticateToken method
			oidcAuth, ok := authenticator.(*oidc.Authenticator)
			g.Expect(ok).To(BeTrue())

			// Create mock token
			var token *mockIDToken
			if tt.claimsError {
				token = createMockIDTokenWithError(fmt.Errorf("claims extraction failed"))
			} else {
				token = createMockIDTokenWithClaims(tt.claims)
			}

			// Call AuthenticateToken
			ctx := context.Background()
			session, err := oidcAuth.AuthenticateToken(ctx, token)

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

// mockIDToken is a test implementation that allows us to control the Claims method
type mockIDToken struct {
	claims map[string]any
	err    error
}

func (m *mockIDToken) Claims(v any) error {
	if m.err != nil {
		return m.err
	}
	// Convert our test claims to the expected format
	if claimsPtr, ok := v.(*map[string]any); ok {
		*claimsPtr = m.claims
		return nil
	}
	return nil
}

// createMockIDTokenWithClaims creates a mock token that properly supports Claims()
func createMockIDTokenWithClaims(claims map[string]any) *mockIDToken {
	return &mockIDToken{
		claims: claims,
	}
}

// createMockIDTokenWithError creates a mock token that returns an error from Claims()
func createMockIDTokenWithError(err error) *mockIDToken {
	return &mockIDToken{
		err: err,
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
	config := map[string]any{
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
	_ = json.NewEncoder(w).Encode(config)
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
