// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

func TestNew(t *testing.T) {
	type contextKey struct{}

	for _, tt := range []struct {
		name            string
		credential      auth.Credential
		provider        auth.Provider
		headers         http.Header
		expectedError   string
		expectedSession *auth.Session
		addContextValue bool
	}{
		{
			name: "successful authentication",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "test-user",
					Password: "test-pass",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "test-user",
					Groups:   []string{"admin"},
					Scopes:   []string{"read", "write"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Basic dGVzdDp0ZXN0"},
			},
			expectedSession: &auth.Session{UserName: "test-user", Groups: []string{"admin"}, Scopes: []string{"read", "write"}},
		},
		{
			name: "credential fails to extract credentials",
			credential: &mockCredentialForMiddleware{
				returnCreds: nil,
				returnOk:    false,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: nil,
				returnError:   nil,
			},
			headers:       http.Header{},
			expectedError: "failed to extract credentials from request",
		},
		{
			name: "authenticator fails",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "invalid-user",
					Password: "invalid-pass",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: nil,
				returnError:   errors.New("authentication failed"),
			},
			headers: http.Header{
				"Authorization": []string{"Basic aW52YWxpZDppbnZhbGlk"},
			},
			expectedError: "failed to authenticate request: authentication failed",
		},
		{
			name: "next handler returns error",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "valid-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "token-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer valid-token"},
			},
			expectedSession: &auth.Session{UserName: "token-user", Groups: []string{"users"}, Scopes: []string{"read"}},
		},
		{
			name: "authentication with custom headers",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "custom-user",
					Token:    "custom-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "custom-user",
					Groups:   []string{"custom", "groups"},
					Scopes:   []string{"admin", "read", "write"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"X-Username": []string{"custom-user"},
				"X-Token":    []string{"custom-token"},
			},
			expectedSession: &auth.Session{UserName: "custom-user", Groups: []string{"custom", "groups"}, Scopes: []string{"admin", "read", "write"}},
		},
		{
			name: "empty credentials extracted but credential succeeds",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "",
					Password: "",
					Token:    "",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "anonymous",
					Groups:   []string{},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			headers: http.Header{
				"X-Custom": []string{"value"},
			},
			expectedSession: &auth.Session{UserName: "anonymous", Groups: []string{}, Scopes: []string{}},
		},
		{
			name: "context propagation test",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "context-user",
				},
				returnOk:           true,
				validateContext:    true,
				expectedContextKey: contextKey{},
				expectedContextVal: "test-value",
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "context-user",
					Groups:   []string{"context"},
					Scopes:   []string{"context-scope"},
				},
				returnError:        nil,
				validateContext:    true,
				expectedContextKey: contextKey{},
				expectedContextVal: "test-value",
			},
			headers: http.Header{
				"Authorization": []string{"Bearer context-token"},
			},
			expectedSession: &auth.Session{UserName: "context-user", Groups: []string{"context"}, Scopes: []string{"context-scope"}},
			addContextValue: true,
		},
		{
			name: "session with empty username and nil groups",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "empty-user-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "",
					Groups:   nil,
					Scopes:   nil,
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer empty-user-token"},
			},
			expectedSession: &auth.Session{UserName: "", Groups: nil, Scopes: nil},
		},
		{
			name: "session with multiple groups",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "multi-group-user",
					Password: "password",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-group-user",
					Groups:   []string{"admin", "users", "developers"},
					Scopes:   []string{"admin", "read", "write", "delete"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Basic bXVsdGktZ3JvdXAtdXNlcjpwYXNzd29yZA=="},
			},
			expectedSession: &auth.Session{UserName: "multi-group-user", Groups: []string{"admin", "users", "developers"}, Scopes: []string{"admin", "read", "write", "delete"}},
		},
		{
			name: "session with empty scopes array",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "empty-scopes-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "empty-scopes-user",
					Groups:   []string{"users"},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer empty-scopes-token"},
			},
			expectedSession: &auth.Session{UserName: "empty-scopes-user", Groups: []string{"users"}, Scopes: []string{}},
		},
		{
			name: "session with single scope",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "single-scope-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "single-scope-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer single-scope-token"},
			},
			expectedSession: &auth.Session{UserName: "single-scope-user", Groups: []string{"users"}, Scopes: []string{"read"}},
		},
		{
			name: "session with complex scopes",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "complex-scopes-user",
					Password: "password",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "complex-scopes-user",
					Groups:   []string{"admin", "users"},
					Scopes:   []string{"user:list", "user:create", "resource:read", "resource:write", "admin:*"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Basic Y29tcGxleC1zY29wZXMtdXNlcjpwYXNzd29yZA=="},
			},
			expectedSession: &auth.Session{UserName: "complex-scopes-user", Groups: []string{"admin", "users"}, Scopes: []string{"user:list", "user:create", "resource:read", "resource:write", "admin:*"}},
		},
		{
			name: "successful authentication with scopes preserved",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Username: "test-user",
					Password: "test-pass",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "test-user",
					Groups:   []string{"admin"},
					Scopes:   []string{"read", "write"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Basic dGVzdDp0ZXN0"},
			},
			expectedSession: &auth.Session{UserName: "test-user", Groups: []string{"admin"}, Scopes: []string{"read", "write"}},
		},
		{
			name: "session with multiple scopes preserved",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "multi-scope-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-scope-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read", "write", "admin", "delete"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer multi-scope-token"},
			},
			expectedSession: &auth.Session{UserName: "multi-scope-user", Groups: []string{"users"}, Scopes: []string{"read", "write", "admin", "delete"}},
		},
		{
			name: "session with empty scopes preserved",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "empty-scopes-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "empty-scopes-user",
					Groups:   []string{"users"},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer empty-scopes-token"},
			},
			expectedSession: &auth.Session{UserName: "empty-scopes-user", Groups: []string{"users"}, Scopes: []string{}},
		},
		{
			name: "session with nil scopes preserved",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "nil-scopes-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "nil-scopes-user",
					Groups:   []string{"users"},
					Scopes:   nil,
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer nil-scopes-token"},
			},
			expectedSession: &auth.Session{UserName: "nil-scopes-user", Groups: []string{"users"}, Scopes: nil},
		},
		{
			name: "session with multiple scopes - test subset validation",
			credential: &mockCredentialForMiddleware{
				returnCreds: &auth.ExtractedCredentials{
					Token: "multi-scope-validation-token",
				},
				returnOk: true,
			},
			provider: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-scope-validation-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read", "write", "user:list", "user:create"},
				},
				returnError: nil,
			},
			headers: http.Header{
				"Authorization": []string{"Bearer multi-scope-validation-token"},
			},
			expectedSession: &auth.Session{UserName: "multi-scope-validation-user", Groups: []string{"users"}, Scopes: []string{"read", "write", "user:list", "user:create"}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create the authenticator
			authenticator := auth.New(tt.credential, tt.provider)

			// Create context for testing
			ctx := context.Background()
			if tt.addContextValue {
				ctx = context.WithValue(ctx, contextKey{}, "test-value")
			}

			// Call the authenticator
			authCtx, err := authenticator(ctx, tt.headers)

			// Verify error expectations
			if tt.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
				g.Expect(authCtx).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(authCtx).NotTo(BeNil())
			}

			// If successful authentication, verify session was added to context
			if tt.expectedError == "" && tt.expectedSession != nil {
				g.Expect(authCtx).NotTo(BeNil())

				// Test FromContext with background context (should fail)
				session := auth.FromContext(context.Background())
				g.Expect(session).To(BeNil())

				// Test CheckScopes with background context (should return nil - no authentication)
				err := auth.CheckScopes(context.Background(), []string{"read"})
				g.Expect(err).NotTo(HaveOccurred())

				// Retrieve the session from the authenticated context
				session = auth.FromContext(authCtx)
				g.Expect(session).NotTo(BeNil())
				g.Expect(session.UserName).To(Equal(tt.expectedSession.UserName))
				g.Expect(session.Groups).To(Equal(tt.expectedSession.Groups))
				g.Expect(session.Scopes).To(Equal(tt.expectedSession.Scopes))

				// Test CheckScopes function with valid context
				if len(tt.expectedSession.Scopes) > 0 {
					// Test with first scope if available
					err = auth.CheckScopes(authCtx, []string{tt.expectedSession.Scopes[0]})
					g.Expect(err).NotTo(HaveOccurred())

					// Test with non-existent scope
					err = auth.CheckScopes(authCtx, []string{"non-existent-scope"})
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("at least one of the following scopes is required: "))

					// Test with multiple scopes if available
					if len(tt.expectedSession.Scopes) > 1 {
						err = auth.CheckScopes(authCtx, []string{tt.expectedSession.Scopes[0], tt.expectedSession.Scopes[1]})
						g.Expect(err).NotTo(HaveOccurred())

						// Test with one valid subset and one invalid subset
						err = auth.CheckScopes(authCtx, []string{tt.expectedSession.Scopes[0], "non-existent"})
						g.Expect(err).NotTo(HaveOccurred()) // Should pass because first subset is valid
					}
				} else if tt.expectedSession.Scopes == nil {
					// When scopes are nil, validation is disabled - should return nil
					err = auth.CheckScopes(authCtx, []string{"any-scope"})
					g.Expect(err).NotTo(HaveOccurred())
				} else {
					// When scopes are empty slice, any scope check should fail
					err = auth.CheckScopes(authCtx, []string{"any-scope"})
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("at least one of the following scopes is required: "))
				}

				// Test empty subset within array (should not return an error - no scopes required)
				err = auth.CheckScopes(authCtx, []string{})
				g.Expect(err).NotTo(HaveOccurred())

				// Verify the expected session structure
				g.Expect(tt.expectedSession).NotTo(BeNil())
				// UserName can be empty in some cases, so just verify it's a string
				g.Expect(tt.expectedSession.UserName).To(BeAssignableToTypeOf(""))
				if tt.expectedSession.Groups != nil {
					for _, group := range tt.expectedSession.Groups {
						g.Expect(group).NotTo(BeEmpty())
					}
				}
			}

			// Verify context validation if enabled
			if mockCredential, ok := tt.credential.(*mockCredentialForMiddleware); ok && mockCredential.validateContext {
				g.Expect(mockCredential.contextReceived).To(BeTrue())
			}
			if mockAuth, ok := tt.provider.(*mockAuthenticatorForMiddleware); ok && mockAuth.validateContext {
				g.Expect(mockAuth.contextReceived).To(BeTrue())
			}
		})
	}
}

func TestNew_Integration(t *testing.T) {
	g := NewWithT(t)

	// Create real credential and authenticator components
	credentialSet := auth.CredentialSet{
		auth.BearerTokenCredential{},
		auth.BasicAuthCredential{},
	}

	providerSet := auth.ProviderSet{
		&mockAuthenticatorForMiddleware{
			returnSession: &auth.Session{
				UserName: "integration-user",
				Groups:   []string{"integration"},
				Scopes:   []string{"integration-scope"},
			},
			returnError: nil,
		},
	}

	authenticator := auth.New(credentialSet, providerSet)

	// Test with bearer token
	headers := http.Header{
		"Authorization": []string{"Bearer integration-token"},
	}

	authCtx, err := authenticator(context.Background(), headers)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(authCtx).NotTo(BeNil())

	// Verify session was added to context
	session := auth.FromContext(authCtx)
	g.Expect(session).NotTo(BeNil())
	g.Expect(session.UserName).To(Equal("integration-user"))
	g.Expect(session.Groups).To(Equal([]string{"integration"}))
	g.Expect(session.Scopes).To(Equal([]string{"integration-scope"}))

	// Test CheckScopes function directly
	scopeCheckError := auth.CheckScopes(authCtx, []string{"integration-scope"})
	g.Expect(scopeCheckError).NotTo(HaveOccurred())
}

// Note: Middleware chaining test removed since the new API (auth.New) returns
// an authenticator function rather than middleware, so middleware chaining is
// no longer applicable to this API.

// Mock credential for middleware testing
type mockCredentialForMiddleware struct {
	returnCreds        *auth.ExtractedCredentials
	returnOk           bool
	validateContext    bool
	expectedContextKey any
	expectedContextVal any
	contextReceived    bool
}

func (m *mockCredentialForMiddleware) Extract(ctx context.Context, header http.Header) (*auth.ExtractedCredentials, bool) {
	if m.validateContext {
		val := ctx.Value(m.expectedContextKey)
		if val == m.expectedContextVal {
			m.contextReceived = true
		}
	}
	return m.returnCreds, m.returnOk
}

// Mock authenticator for middleware testing
type mockAuthenticatorForMiddleware struct {
	returnSession      *auth.Session
	returnError        error
	validateContext    bool
	expectedContextKey any
	expectedContextVal any
	contextReceived    bool
}

func (m *mockAuthenticatorForMiddleware) Authenticate(ctx context.Context, credentials auth.ExtractedCredentials) (*auth.Session, error) {
	if m.validateContext {
		val := ctx.Value(m.expectedContextKey)
		if val == m.expectedContextVal {
			m.contextReceived = true
		}
	}
	return m.returnSession, m.returnError
}
