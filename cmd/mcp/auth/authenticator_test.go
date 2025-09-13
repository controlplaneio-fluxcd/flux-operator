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

func TestAuthenticatorSet_Authenticate(t *testing.T) {
	for _, tt := range []struct {
		name                string
		authenticators      []auth.Authenticator
		credentials         auth.Credentials
		expectedSession     *auth.Session
		expectError         bool
		expectedErrorSubstr string
	}{
		{
			name: "first authenticator succeeds",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: &auth.Session{UserName: "user1", Groups: []string{"group1"}},
					err:     nil,
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("second authenticator error"),
				},
			},
			credentials: auth.Credentials{
				Username: "test-user",
				Password: "test-pass",
				Token:    "test-token",
			},
			expectedSession: &auth.Session{UserName: "user1", Groups: []string{"group1"}},
		},
		{
			name: "second authenticator succeeds after first fails",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("first authenticator failed"),
				},
				&mockAuthenticator{
					session: &auth.Session{UserName: "user2", Groups: []string{"group2", "group3"}},
					err:     nil,
				},
			},
			credentials: auth.Credentials{
				Username: "test-user",
				Password: "test-pass",
			},
			expectedSession: &auth.Session{UserName: "user2", Groups: []string{"group2", "group3"}},
		},
		{
			name: "third authenticator succeeds after two fail",
			authenticators: []auth.Authenticator{
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
			credentials: auth.Credentials{
				Token: "valid-token",
			},
			expectedSession: &auth.Session{UserName: "user3", Groups: []string{}},
		},
		{
			name: "all authenticators fail",
			authenticators: []auth.Authenticator{
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
			credentials: auth.Credentials{
				Username: "bad-user",
				Password: "bad-pass",
			},
			expectError:         true,
			expectedErrorSubstr: "no authenticators succeeded",
		},
		{
			name: "single authenticator succeeds",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: &auth.Session{UserName: "single-user", Groups: []string{"admin", "users"}},
					err:     nil,
				},
			},
			credentials: auth.Credentials{
				Username: "single-user",
				Token:    "bearer-token",
			},
			expectedSession: &auth.Session{UserName: "single-user", Groups: []string{"admin", "users"}},
		},
		{
			name: "single authenticator fails",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("authentication failed"),
				},
			},
			credentials: auth.Credentials{
				Username: "bad-user",
			},
			expectError:         true,
			expectedErrorSubstr: "no authenticators succeeded",
		},
		{
			name:                "empty authenticator set",
			authenticators:      []auth.Authenticator{},
			credentials:         auth.Credentials{Username: "user"},
			expectError:         true,
			expectedErrorSubstr: "no authenticators succeeded",
		},
		{
			name: "authenticator returns nil session with nil error",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: nil,
					err:     nil,
				},
			},
			credentials: auth.Credentials{
				Username: "test-user",
			},
			expectedSession: nil,
		},
		{
			name: "mixed success and failure with different error types",
			authenticators: []auth.Authenticator{
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
			credentials: auth.Credentials{
				Password: "password",
				Token:    "token",
			},
			expectedSession: &auth.Session{UserName: "final-user", Groups: []string{"final-group"}},
		},
		{
			name: "error aggregation when all fail",
			authenticators: []auth.Authenticator{
				&mockAuthenticator{
					session: nil,
					err:     errors.New("error1"),
				},
				&mockAuthenticator{
					session: nil,
					err:     errors.New("error2"),
				},
			},
			credentials: auth.Credentials{
				Username: "test",
			},
			expectError:         true,
			expectedErrorSubstr: "no authenticators succeeded",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create the AuthenticatorSet
			authSet := auth.AuthenticatorSet(tt.authenticators)

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

func TestAuthenticatorSet_Authenticate_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	type contextKey struct{}

	// Create a mock authenticator that checks if context is properly passed
	mockAuth := &mockAuthenticatorWithContext{
		expectedContextKey: contextKey{},
		expectedContextVal: "test-value",
		session:            &auth.Session{UserName: "context-user"},
	}

	authSet := auth.AuthenticatorSet{mockAuth}

	// Create context with test data
	ctx := context.WithValue(context.Background(), contextKey{}, "test-value")

	session, err := authSet.Authenticate(ctx, auth.Credentials{Username: "test"})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(session).NotTo(BeNil())
	g.Expect(session.UserName).To(Equal("context-user"))
	g.Expect(mockAuth.contextReceived).To(BeTrue())
}

func TestAuthenticatorSet_Authenticate_CredentialsPropagation(t *testing.T) {
	g := NewWithT(t)

	expectedCreds := auth.Credentials{
		Username: "test-user",
		Password: "test-password",
		Token:    "test-token",
	}

	mockAuth := &mockAuthenticatorWithCredentials{
		expectedCredentials: expectedCreds,
		session:             &auth.Session{UserName: "creds-user"},
	}

	authSet := auth.AuthenticatorSet{mockAuth}

	session, err := authSet.Authenticate(context.Background(), expectedCreds)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(session).NotTo(BeNil())
	g.Expect(session.UserName).To(Equal("creds-user"))
	g.Expect(mockAuth.credentialsReceived).To(BeTrue())
}

// Mock authenticator for testing
type mockAuthenticator struct {
	session *auth.Session
	err     error
}

func (m *mockAuthenticator) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.Session, error) {
	return m.session, m.err
}

// Mock authenticator that validates context propagation
type mockAuthenticatorWithContext struct {
	expectedContextKey any
	expectedContextVal any
	session            *auth.Session
	contextReceived    bool
}

func (m *mockAuthenticatorWithContext) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.Session, error) {
	val := ctx.Value(m.expectedContextKey)
	if val == m.expectedContextVal {
		m.contextReceived = true
	}
	return m.session, nil
}

// Mock authenticator that validates credentials propagation
type mockAuthenticatorWithCredentials struct {
	expectedCredentials auth.Credentials
	session             *auth.Session
	credentialsReceived bool
}

func (m *mockAuthenticatorWithCredentials) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.Session, error) {
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
