// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

func TestProviderSet_Authenticate(t *testing.T) {
	for _, tt := range []struct {
		name                string
		providers           []auth.Provider
		credentials         auth.ExtractedCredentials
		expectedSession     *auth.Session
		expectError         bool
		expectedErrorSubstr string
	}{
		{
			name: "first provider succeeds",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: &auth.Session{UserName: "user1", Groups: []string{"group1"}},
					err:     nil,
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("second authenticator error"),
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "test-user",
				Password: "test-pass",
				Token:    "test-token",
			},
			expectedSession: &auth.Session{UserName: "user1", Groups: []string{"group1"}},
		},
		{
			name: "second provider succeeds after first fails",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("first authenticator failed"),
				},
				&mockAuthenticator{
					session: &auth.Session{UserName: "user2", Groups: []string{"group2", "group3"}},
					err:     nil,
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "test-user",
				Password: "test-pass",
			},
			expectedSession: &auth.Session{UserName: "user2", Groups: []string{"group2", "group3"}},
		},
		{
			name: "third provider succeeds after two fail",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("first auth failed"),
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("second auth failed"),
				},
				&mockAuthenticator{
					session: &auth.Session{UserName: "user3", Groups: []string{}},
					err:     nil,
				},
			},
			credentials: auth.ExtractedCredentials{
				Token: "valid-token",
			},
			expectedSession: &auth.Session{UserName: "user3", Groups: []string{}},
		},
		{
			name: "all providers fail",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("first failed"),
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("second failed"),
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("third failed"),
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "bad-user",
				Password: "bad-pass",
			},
			expectError:         true,
			expectedErrorSubstr: "no providers succeeded",
		},
		{
			name: "single provider succeeds",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: &auth.Session{UserName: "single-user", Groups: []string{"admin", "users"}},
					err:     nil,
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "single-user",
				Token:    "bearer-token",
			},
			expectedSession: &auth.Session{UserName: "single-user", Groups: []string{"admin", "users"}},
		},
		{
			name: "single provider fails",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("authentication failed"),
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "bad-user",
			},
			expectError:         true,
			expectedErrorSubstr: "no providers succeeded",
		},
		{
			name:                "empty provider set",
			providers:           []auth.Provider{},
			credentials:         auth.ExtractedCredentials{Username: "user"},
			expectError:         true,
			expectedErrorSubstr: "no providers succeeded",
		},
		{
			name: "provider returns nil session with nil error",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     nil,
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "test-user",
			},
			expectedSession: nil,
		},
		{
			name: "mixed success and failure with different error types",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     &customError{msg: "custom auth error"},
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("standard error"),
				},
				&mockAuthenticator{
					session: &auth.Session{UserName: "final-user", Groups: []string{"final-group"}},
					err:     nil,
				},
			},
			credentials: auth.ExtractedCredentials{
				Password: "password",
				Token:    "token",
			},
			expectedSession: &auth.Session{UserName: "final-user", Groups: []string{"final-group"}},
		},
		{
			name: "error aggregation when all fail",
			providers: []auth.Provider{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("error1"),
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("error2"),
				},
			},
			credentials: auth.ExtractedCredentials{
				Username: "test",
			},
			expectError:         true,
			expectedErrorSubstr: "no providers succeeded",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create the ProviderSet
			authSet := auth.ProviderSet(tt.providers)

			// Call Authenticate
			ctx := context.Background()
			session, err := authSet.Authenticate(ctx, tt.credentials)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErrorSubstr))
				g.Expect(session).To(BeNil())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			if tt.expectedSession == nil {
				g.Expect(session).To(BeNil())
			} else {
				g.Expect(session).NotTo(BeNil())
				g.Expect(session.UserName).To(Equal(tt.expectedSession.UserName))
				g.Expect(session.Groups).To(Equal(tt.expectedSession.Groups))
			}
		})
	}
}

func TestProviderSet_Authenticate_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	type contextKey struct{}

	// Create a mock provider that checks if context is properly passed
	mockAuth := &mockAuthenticatorWithContext{
		expectedContextKey: contextKey{},
		expectedContextVal: "test-value",
		session:            &auth.Session{UserName: "context-user"},
	}

	authSet := auth.ProviderSet{mockAuth}

	// Create context with test data
	ctx := context.WithValue(context.Background(), contextKey{}, "test-value")

	session, err := authSet.Authenticate(ctx, auth.ExtractedCredentials{Username: "test"})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(session).NotTo(BeNil())
	g.Expect(session.UserName).To(Equal("context-user"))
	g.Expect(mockAuth.contextReceived).To(BeTrue())
}

func TestProviderSet_Authenticate_CredentialsPropagation(t *testing.T) {
	g := NewWithT(t)

	expectedCreds := auth.ExtractedCredentials{
		Username: "test-user",
		Password: "test-password",
		Token:    "test-token",
	}

	mockAuth := &mockAuthenticatorWithCredentials{
		expectedCredentials: expectedCreds,
		session:             &auth.Session{UserName: "creds-user"},
	}

	authSet := auth.ProviderSet{mockAuth}

	session, err := authSet.Authenticate(context.Background(), expectedCreds)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(session).NotTo(BeNil())
	g.Expect(session.UserName).To(Equal("creds-user"))
	g.Expect(mockAuth.credentialsReceived).To(BeTrue())
}

// Mock provider for testing
type mockAuthenticator struct {
	session *auth.Session
	err     error
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, credentials auth.ExtractedCredentials) (*auth.Session, error) {
	return m.session, m.err
}

// Mock provider that validates context propagation
type mockAuthenticatorWithContext struct {
	expectedContextKey any
	expectedContextVal any
	session            *auth.Session
	contextReceived    bool
}

func (m *mockAuthenticatorWithContext) Authenticate(ctx context.Context, credentials auth.ExtractedCredentials) (*auth.Session, error) {
	val := ctx.Value(m.expectedContextKey)
	if val == m.expectedContextVal {
		m.contextReceived = true
	}
	return m.session, nil
}

// Mock provider that validates credentials propagation
type mockAuthenticatorWithCredentials struct {
	expectedCredentials auth.ExtractedCredentials
	session             *auth.Session
	credentialsReceived bool
}

func (m *mockAuthenticatorWithCredentials) Authenticate(ctx context.Context, credentials auth.ExtractedCredentials) (*auth.Session, error) {
	if credentials.Username == m.expectedCredentials.Username &&
		credentials.Password == m.expectedCredentials.Password &&
		credentials.Token == m.expectedCredentials.Token {
		m.credentialsReceived = true
	}
	return m.session, nil
}

// Custom error type for testing different error handling
type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}
