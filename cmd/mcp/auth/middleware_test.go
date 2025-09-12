// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

func TestNewMiddleware(t *testing.T) {
	type contextKey struct{}

	for _, tt := range []struct {
		name             string
		transport        auth.Transport
		authenticator    auth.Authenticator
		request          mcp.CallToolRequest
		expectedError    string
		expectedSession  *auth.Session
		expectNextCalled bool
		nextHandlerError error
		expectedResult   *mcp.CallToolResult
		addContextValue  bool
		validateScopes   bool
	}{
		{
			name: "successful authentication",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "test-user",
					Password: "test-pass",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "test-user",
					Groups:   []string{"admin"},
					Scopes:   []string{"read", "write"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Basic dGVzdDp0ZXN0"},
				},
			},
			expectedSession:  &auth.Session{UserName: "test-user", Groups: []string{"admin"}, Scopes: []string{"read", "write"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "transport fails to extract credentials",
			transport: &mockTransportForMiddleware{
				returnCreds: nil,
				returnOk:    false,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: nil,
				returnError:   nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{},
			},
			expectedError:    "failed to extract credentials from request",
			expectNextCalled: false,
		},
		{
			name: "authenticator fails",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "invalid-user",
					Password: "invalid-pass",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: nil,
				returnError:   errors.New("authentication failed"),
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Basic aW52YWxpZDppbnZhbGlk"},
				},
			},
			expectedError:    "failed to authenticate request: authentication failed",
			expectNextCalled: false,
		},
		{
			name: "next handler returns error",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "valid-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "token-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer valid-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "token-user", Groups: []string{"users"}, Scopes: []string{"read"}},
			expectNextCalled: true,
			nextHandlerError: errors.New("next handler error"),
			expectedError:    "next handler error",
			validateScopes:   true,
		},
		{
			name: "authentication with custom headers",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "custom-user",
					Token:    "custom-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "custom-user",
					Groups:   []string{"custom", "groups"},
					Scopes:   []string{"admin", "read", "write"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"X-Username": []string{"custom-user"},
					"X-Token":    []string{"custom-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "custom-user", Groups: []string{"custom", "groups"}, Scopes: []string{"admin", "read", "write"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "empty credentials extracted but transport succeeds",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "",
					Password: "",
					Token:    "",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "anonymous",
					Groups:   []string{},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"X-Custom": []string{"value"},
				},
			},
			expectedSession:  &auth.Session{UserName: "anonymous", Groups: []string{}, Scopes: []string{}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "context propagation test",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "context-user",
				},
				returnOk:           true,
				validateContext:    true,
				expectedContextKey: contextKey{},
				expectedContextVal: "test-value",
			},
			authenticator: &mockAuthenticatorForMiddleware{
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
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer context-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "context-user", Groups: []string{"context"}, Scopes: []string{"context-scope"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			addContextValue:  true,
			validateScopes:   true,
		},
		{
			name: "session with empty username and nil groups",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "empty-user-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "",
					Groups:   nil,
					Scopes:   nil,
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer empty-user-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "", Groups: nil, Scopes: []string{}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "session with multiple groups",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "multi-group-user",
					Password: "password",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-group-user",
					Groups:   []string{"admin", "users", "developers"},
					Scopes:   []string{"admin", "read", "write", "delete"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Basic bXVsdGktZ3JvdXAtdXNlcjpwYXNzd29yZA=="},
				},
			},
			expectedSession:  &auth.Session{UserName: "multi-group-user", Groups: []string{"admin", "users", "developers"}, Scopes: []string{"admin", "read", "write", "delete"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "session with empty scopes array",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "empty-scopes-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "empty-scopes-user",
					Groups:   []string{"users"},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer empty-scopes-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "empty-scopes-user", Groups: []string{"users"}, Scopes: []string{}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "session with single scope",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "single-scope-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "single-scope-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer single-scope-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "single-scope-user", Groups: []string{"users"}, Scopes: []string{"read"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "session with complex scopes",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "complex-scopes-user",
					Password: "password",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "complex-scopes-user",
					Groups:   []string{"admin", "users"},
					Scopes:   []string{"user:list", "user:create", "resource:read", "resource:write", "admin:*"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Basic Y29tcGxleC1zY29wZXMtdXNlcjpwYXNzd29yZA=="},
				},
			},
			expectedSession:  &auth.Session{UserName: "complex-scopes-user", Groups: []string{"admin", "users"}, Scopes: []string{"user:list", "user:create", "resource:read", "resource:write", "admin:*"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
		{
			name: "successful authentication with validateScopes=false",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Username: "test-user",
					Password: "test-pass",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "test-user",
					Groups:   []string{"admin"},
					Scopes:   []string{"read", "write"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Basic dGVzdDp0ZXN0"},
				},
			},
			expectedSession:  &auth.Session{UserName: "test-user", Groups: []string{"admin"}, Scopes: nil},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   false,
		},
		{
			name: "session with multiple scopes but validateScopes=false",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "multi-scope-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-scope-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read", "write", "admin", "delete"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer multi-scope-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "multi-scope-user", Groups: []string{"users"}, Scopes: nil},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   false,
		},
		{
			name: "session with empty scopes and validateScopes=false",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "empty-scopes-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "empty-scopes-user",
					Groups:   []string{"users"},
					Scopes:   []string{},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer empty-scopes-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "empty-scopes-user", Groups: []string{"users"}, Scopes: nil},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   false,
		},
		{
			name: "session with nil scopes and validateScopes=false",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "nil-scopes-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "nil-scopes-user",
					Groups:   []string{"users"},
					Scopes:   nil,
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer nil-scopes-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "nil-scopes-user", Groups: []string{"users"}, Scopes: nil},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   false,
		},
		{
			name: "session with multiple scopes - test subset validation",
			transport: &mockTransportForMiddleware{
				returnCreds: &auth.Credentials{
					Token: "multi-scope-validation-token",
				},
				returnOk: true,
			},
			authenticator: &mockAuthenticatorForMiddleware{
				returnSession: &auth.Session{
					UserName: "multi-scope-validation-user",
					Groups:   []string{"users"},
					Scopes:   []string{"read", "write", "user:list", "user:create"},
				},
				returnError: nil,
			},
			request: mcp.CallToolRequest{
				Header: http.Header{
					"Authorization": []string{"Bearer multi-scope-validation-token"},
				},
			},
			expectedSession:  &auth.Session{UserName: "multi-scope-validation-user", Groups: []string{"users"}, Scopes: []string{"read", "write", "user:list", "user:create"}},
			expectNextCalled: true,
			expectedResult:   mcp.NewToolResultText("success"),
			validateScopes:   true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create the middleware
			middleware := auth.NewMiddleware(tt.transport, tt.authenticator, tt.validateScopes)

			// Create a mock next handler
			var nextCalled bool
			var receivedContext context.Context
			nextHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				nextCalled = true
				receivedContext = ctx

				if tt.nextHandlerError != nil {
					return nil, tt.nextHandlerError
				}

				return tt.expectedResult, nil
			}

			// Apply middleware
			wrappedHandler := middleware(nextHandler)

			// Create context for testing
			ctx := context.Background()
			if tt.addContextValue {
				ctx = context.WithValue(ctx, contextKey{}, "test-value")
			}

			// Call the wrapped handler
			result, err := wrappedHandler(ctx, tt.request)

			// Verify error expectations
			if tt.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				if tt.expectedResult != nil {
					g.Expect(result).To(Equal(tt.expectedResult))
				}
			}

			// Verify next handler was called as expected
			g.Expect(nextCalled).To(Equal(tt.expectNextCalled))

			// If next was called, verify session was added to context
			if tt.expectNextCalled && tt.expectedSession != nil {
				g.Expect(receivedContext).NotTo(BeNil())

				// Test FromContext with background context (should fail)
				session := auth.FromContext(context.Background())
				g.Expect(session).To(BeNil())

				// Test CheckScopes with background context (should return nil - no authentication)
				err := auth.CheckScopes(context.Background(), []string{"read"})
				g.Expect(err).NotTo(HaveOccurred())

				// Now that the middleware bug is fixed, we should be able to retrieve the session
				session = auth.FromContext(receivedContext)
				g.Expect(session).NotTo(BeNil())
				g.Expect(session.UserName).To(Equal(tt.expectedSession.UserName))
				g.Expect(session.Groups).To(Equal(tt.expectedSession.Groups))
				g.Expect(session.Scopes).To(Equal(tt.expectedSession.Scopes))

				// Test CheckScopes function with valid context
				if len(tt.expectedSession.Scopes) > 0 {
					// Test with first scope if available
					err = auth.CheckScopes(receivedContext, []string{tt.expectedSession.Scopes[0]})
					g.Expect(err).NotTo(HaveOccurred())

					// Test with non-existent scope
					err = auth.CheckScopes(receivedContext, []string{"non-existent-scope"})
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("at least one of the following scopes is required: "))

					// Test with multiple scopes if available
					if len(tt.expectedSession.Scopes) > 1 {
						err = auth.CheckScopes(receivedContext, []string{tt.expectedSession.Scopes[0], tt.expectedSession.Scopes[1]})
						g.Expect(err).NotTo(HaveOccurred())

						// Test with one valid subset and one invalid subset
						err = auth.CheckScopes(receivedContext, []string{tt.expectedSession.Scopes[0], "non-existent"})
						g.Expect(err).NotTo(HaveOccurred()) // Should pass because first subset is valid
					}
				} else if tt.expectedSession.Scopes == nil {
					// When scopes are nil, validation is disabled - should return nil
					err = auth.CheckScopes(receivedContext, []string{"any-scope"})
					g.Expect(err).NotTo(HaveOccurred())
				} else {
					// When scopes are empty slice, any scope check should fail
					err = auth.CheckScopes(receivedContext, []string{"any-scope"})
					g.Expect(err).To(HaveOccurred())
					g.Expect(err.Error()).To(ContainSubstring("at least one of the following scopes is required: "))
				}

				// Test empty subset within array
				err = auth.CheckScopes(receivedContext, []string{})
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("cannot check empty set of scopes"))

				// Verify the expected session structure
				g.Expect(tt.expectedSession).NotTo(BeNil())
				// UserName can be empty in some cases, so just verify it's a string
				g.Expect(tt.expectedSession.UserName).To(BeAssignableToTypeOf(""))
				if tt.expectedSession.Groups != nil {
					for _, group := range tt.expectedSession.Groups {
						g.Expect(group).NotTo(BeEmpty())
					}
				}
			} else if tt.expectNextCalled {
				g.Expect(receivedContext).NotTo(BeNil())
				g.Expect(receivedContext).NotTo(Equal(context.Background()))
			}

			// Verify context validation if enabled
			if mockTransport, ok := tt.transport.(*mockTransportForMiddleware); ok && mockTransport.validateContext {
				g.Expect(mockTransport.contextReceived).To(BeTrue())
			}
			if mockAuth, ok := tt.authenticator.(*mockAuthenticatorForMiddleware); ok && mockAuth.validateContext {
				g.Expect(mockAuth.contextReceived).To(BeTrue())
			}
		})
	}
}

func TestNewMiddleware_Integration(t *testing.T) {
	g := NewWithT(t)

	// Create real transport and authenticator components
	transportSet := auth.TransportSet{
		auth.BearerTokenTransport{},
		auth.BasicAuthTransport{},
	}

	authenticatorSet := auth.AuthenticatorSet{
		&mockAuthenticatorForMiddleware{
			returnSession: &auth.Session{
				UserName: "integration-user",
				Groups:   []string{"integration"},
				Scopes:   []string{"integration-scope"},
			},
			returnError: nil,
		},
	}

	middleware := auth.NewMiddleware(transportSet, authenticatorSet, true)

	// Test with bearer token
	var sessionFromNext *auth.Session
	var scopeCheckError error
	nextHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		session := auth.FromContext(ctx)
		if session != nil {
			sessionFromNext = session
		}
		scopeCheckError = auth.CheckScopes(ctx, []string{"integration-scope"})
		return mcp.NewToolResultText("integration success"), nil
	}

	wrappedHandler := middleware(nextHandler)

	request := mcp.CallToolRequest{
		Header: http.Header{
			"Authorization": []string{"Bearer integration-token"},
		},
	}

	result, err := wrappedHandler(context.Background(), request)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).NotTo(BeNil())
	textContent, ok := mcp.AsTextContent(result.Content[0])
	g.Expect(ok).To(BeTrue())
	g.Expect(textContent.Text).To(Equal("integration success"))
	g.Expect(sessionFromNext).NotTo(BeNil())
	g.Expect(sessionFromNext.UserName).To(Equal("integration-user"))
	g.Expect(sessionFromNext.Groups).To(Equal([]string{"integration"}))
	g.Expect(sessionFromNext.Scopes).To(Equal([]string{"integration-scope"}))

	// Test CheckScopes function directly
	g.Expect(scopeCheckError).NotTo(HaveOccurred())
}

func TestNewMiddleware_MiddlewareChaining(t *testing.T) {
	g := NewWithT(t)

	// Test that middleware can be chained
	transport := &mockTransportForMiddleware{
		returnCreds: &auth.Credentials{Username: "chain-user"},
		returnOk:    true,
	}

	authenticator := &mockAuthenticatorForMiddleware{
		returnSession: &auth.Session{
			UserName: "chain-user",
			Groups:   []string{"chain"},
			Scopes:   []string{"chain-scope"},
		},
		returnError: nil,
	}

	authMiddleware := auth.NewMiddleware(transport, authenticator, true)

	// Another middleware that adds a custom header
	customMiddleware := func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := next(ctx, request)
			if err != nil {
				return nil, err
			}
			// Get original text and append custom text
			if len(result.Content) > 0 {
				if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
					combinedText := textContent.Text + " | custom middleware applied"
					return mcp.NewToolResultText(combinedText), nil
				}
			}
			return result, nil
		}
	}

	finalHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Now that the middleware bug is fixed, we can properly retrieve the session
		session := auth.FromContext(ctx)
		if session == nil {
			return nil, errors.New("no session in context")
		}
		return mcp.NewToolResultText("handler executed for " + session.UserName), nil
	}

	// Chain middlewares: auth -> custom -> final
	chainedHandler := authMiddleware(customMiddleware(finalHandler))

	request := mcp.CallToolRequest{
		Header: http.Header{
			"Authorization": []string{"Bearer chain-token"},
		},
	}

	result, err := chainedHandler(context.Background(), request)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).NotTo(BeNil())
	g.Expect(result.Content).To(HaveLen(1))
	textContent, ok := mcp.AsTextContent(result.Content[0])
	g.Expect(ok).To(BeTrue())
	g.Expect(textContent.Text).To(Equal("handler executed for chain-user | custom middleware applied"))
}

// Mock transport for middleware testing
type mockTransportForMiddleware struct {
	returnCreds        *auth.Credentials
	returnOk           bool
	validateContext    bool
	expectedContextKey any
	expectedContextVal any
	contextReceived    bool
}

func (m *mockTransportForMiddleware) Extract(ctx context.Context, header http.Header) (*auth.Credentials, bool) {
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

func (m *mockAuthenticatorForMiddleware) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.Session, error) {
	if m.validateContext {
		val := ctx.Value(m.expectedContextKey)
		if val == m.expectedContextVal {
			m.contextReceived = true
		}
	}
	return m.returnSession, m.returnError
}
