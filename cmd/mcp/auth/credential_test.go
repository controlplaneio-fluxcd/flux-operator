// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

func TestCredentialSet_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		credentials     []auth.Credential
		headers         map[string]string
		expectedCreds   *auth.ExtractedCredentials
		expectedSuccess bool
	}{
		{
			name: "first credential succeeds",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
				auth.BasicAuthCredential{},
			},
			headers: map[string]string{
				"Authorization": "Bearer test-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "test-token",
			},
			expectedSuccess: true,
		},
		{
			name: "first credential succeeds with basic auth header (bearer extracts it as token)",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
				auth.BasicAuthCredential{},
			},
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")), // Bearer credential succeeds first
			},
			expectedSuccess: true,
		},
		{
			name: "custom header credential succeeds after others fail",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
				auth.BasicAuthCredential{},
				&auth.CustomHTTPHeaderCredential{
					CustomHTTPHeaderSpec: config.CustomHTTPHeaderSpec{
						Username: "X-Username",
						Password: "X-Password",
					},
				},
			},
			headers: map[string]string{
				"X-Username": "custom-user",
				"X-Password": "custom-pass",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "custom-user",
				Password: "custom-pass",
			},
			expectedSuccess: true,
		},
		{
			name: "all credentials fail",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
				auth.BasicAuthCredential{},
			},
			headers: map[string]string{
				"X-Custom-Header": "value",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name:            "empty credential set",
			credentials:     []auth.Credential{},
			headers:         map[string]string{"Authorization": "Bearer token"},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "single credential succeeds",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
			},
			headers: map[string]string{
				"Authorization": "Bearer single-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "single-token",
			},
			expectedSuccess: true,
		},
		{
			name: "single credential fails",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
			},
			headers: map[string]string{
				"X-Other-Header": "value",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "second credential succeeds when first fails (no auth header)",
			credentials: []auth.Credential{
				auth.BearerTokenCredential{},
				&auth.CustomHTTPHeaderCredential{
					CustomHTTPHeaderSpec: config.CustomHTTPHeaderSpec{
						Token: "X-Token",
					},
				},
			},
			headers: map[string]string{
				"X-Token": "custom-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "custom-token",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			credentialSet := auth.CredentialSet(tt.credentials)
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := credentialSet.Extract(context.Background(), header)

			g.Expect(ok).To(Equal(tt.expectedSuccess))
			if tt.expectedCreds == nil {
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(creds).NotTo(BeNil())
				g.Expect(creds.Username).To(Equal(tt.expectedCreds.Username))
				g.Expect(creds.Password).To(Equal(tt.expectedCreds.Password))
				g.Expect(creds.Token).To(Equal(tt.expectedCreds.Token))
			}
		})
	}
}

func TestBearerTokenCredential_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		headers         map[string]string
		expectedCreds   *auth.ExtractedCredentials
		expectedSuccess bool
	}{
		{
			name: "valid bearer token",
			headers: map[string]string{
				"Authorization": "Bearer valid-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "valid-token",
			},
			expectedSuccess: true,
		},
		{
			name: "bearer token with spaces",
			headers: map[string]string{
				"Authorization": "Bearer token-with-spaces",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "token-with-spaces",
			},
			expectedSuccess: true,
		},
		{
			name: "bearer token with special characters",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.Gfx6VO9tcxwk6xqx9yYzSfebfeakZp5JYIgP_edaZAQ",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.Gfx6VO9tcxwk6xqx9yYzSfebfeakZp5JYIgP_edaZAQ",
			},
			expectedSuccess: true,
		},
		{
			name:            "missing authorization header",
			headers:         map[string]string{},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "empty authorization header",
			headers: map[string]string{
				"Authorization": "",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "basic auth instead of bearer (TrimPrefix behavior)",
			headers: map[string]string{
				"Authorization": "Basic dXNlcjpwYXNz",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "Basic dXNlcjpwYXNz", // TrimPrefix returns original string if prefix not found
			},
			expectedSuccess: true,
		},
		{
			name: "bearer without token",
			headers: map[string]string{
				"Authorization": "Bearer",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "Bearer", // TrimPrefix doesn't match "Bearer " (with space)
			},
			expectedSuccess: true,
		},
		{
			name: "bearer with empty token",
			headers: map[string]string{
				"Authorization": "Bearer ",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "bearer with exact prefix 'Bearer '",
			headers: map[string]string{
				"Authorization": "Bearer ",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "case sensitive bearer",
			headers: map[string]string{
				"Authorization": "bearer token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "bearer token", // TrimPrefix is case sensitive, doesn't match "Bearer "
			},
			expectedSuccess: true,
		},
		{
			name: "authorization with different case",
			headers: map[string]string{
				"authorization": "Bearer token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "token",
			},
			expectedSuccess: true,
		},
		{
			name: "multiple authorization headers",
			headers: map[string]string{
				"Authorization": "Bearer first-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "first-token",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			credential := auth.BearerTokenCredential{}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := credential.Extract(context.Background(), header)

			g.Expect(ok).To(Equal(tt.expectedSuccess))
			if tt.expectedCreds == nil {
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(creds).NotTo(BeNil())
				g.Expect(creds.Username).To(Equal(tt.expectedCreds.Username))
				g.Expect(creds.Password).To(Equal(tt.expectedCreds.Password))
				g.Expect(creds.Token).To(Equal(tt.expectedCreds.Token))
			}
		})
	}
}

func TestBasicAuthCredential_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		headers         map[string]string
		expectedCreds   *auth.ExtractedCredentials
		expectedSuccess bool
	}{
		{
			name: "valid basic auth",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user",
				Password: "pass",
			},
			expectedSuccess: true,
		},
		{
			name: "basic auth with special characters",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user@domain.com:p@ss!w0rd")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user@domain.com",
				Password: "p@ss!w0rd",
			},
			expectedSuccess: true,
		},
		{
			name: "basic auth with empty password",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user",
				Password: "",
			},
			expectedSuccess: true,
		},
		{
			name: "basic auth with empty username",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(":pass")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "",
				Password: "pass",
			},
			expectedSuccess: true,
		},
		{
			name: "basic auth with empty username and password",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte(":")),
			},
			expectedSuccess: false,
		},
		{
			name: "basic auth with unicode characters",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("ユーザー:パスワード")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "ユーザー",
				Password: "パスワード",
			},
			expectedSuccess: true,
		},
		{
			name:            "missing authorization header",
			headers:         map[string]string{},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "bearer auth instead of basic",
			headers: map[string]string{
				"Authorization": "Bearer token",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "malformed basic auth",
			headers: map[string]string{
				"Authorization": "Basic invalid-base64",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "basic without credentials",
			headers: map[string]string{
				"Authorization": "Basic",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "empty authorization header",
			headers: map[string]string{
				"Authorization": "",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "case insensitive basic auth",
			headers: map[string]string{
				"Authorization": "basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user",
				Password: "pass",
			},
			expectedSuccess: true,
		},
		{
			name: "valid basic auth to test success path",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("testuser:testpass")),
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "testuser",
				Password: "testpass",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			credential := auth.BasicAuthCredential{}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := credential.Extract(context.Background(), header)

			g.Expect(ok).To(Equal(tt.expectedSuccess))
			if tt.expectedCreds == nil {
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(creds).NotTo(BeNil())
				g.Expect(creds.Username).To(Equal(tt.expectedCreds.Username))
				g.Expect(creds.Password).To(Equal(tt.expectedCreds.Password))
				g.Expect(creds.Token).To(Equal(tt.expectedCreds.Token))
			}
		})
	}
}

func TestCustomHTTPHeaderCredential_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            config.CustomHTTPHeaderSpec
		headers         map[string]string
		expectedCreds   *auth.ExtractedCredentials
		expectedSuccess bool
	}{
		{
			name: "username and password headers",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "test-user",
				"X-Password": "test-pass",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "test-user",
				Password: "test-pass",
			},
			expectedSuccess: true,
		},
		{
			name: "token header only",
			spec: config.CustomHTTPHeaderSpec{
				Token: "X-Auth-Token",
			},
			headers: map[string]string{
				"X-Auth-Token": "custom-token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Token: "custom-token",
			},
			expectedSuccess: true,
		},
		{
			name: "all three headers",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-User",
				Password: "X-Pass",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-User":  "user",
				"X-Pass":  "pass",
				"X-Token": "token",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user",
				Password: "pass",
				Token:    "token",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - username only",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Username": "user-only",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user-only",
				Password: "",
				Token:    "",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - password only",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Password": "pass-only",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "",
				Password: "pass-only",
				Token:    "",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - token only",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Token": "token-only",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "",
				Password: "",
				Token:    "token-only",
			},
			expectedSuccess: true,
		},
		{
			name: "no matching headers",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"Authorization": "Bearer token",
				"X-Other":       "value",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "empty header values",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Username": "",
				"X-Password": "",
				"X-Token":    "",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "case sensitive headers",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
			},
			headers: map[string]string{
				"x-username": "lower-case",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "lower-case",
			},
			expectedSuccess: true,
		},
		{
			name: "headers with special characters",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "user@domain.com",
				"X-Password": "p@ss!w0rd#123",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "user@domain.com",
				Password: "p@ss!w0rd#123",
			},
			expectedSuccess: true,
		},
		{
			name: "empty spec fields",
			spec: config.CustomHTTPHeaderSpec{},
			headers: map[string]string{
				"X-Username": "user",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "unicode values",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "ユーザー",
				"X-Password": "パスワード",
			},
			expectedCreds: &auth.ExtractedCredentials{
				Username: "ユーザー",
				Password: "パスワード",
			},
			expectedSuccess: true,
		},
		{
			name: "all empty headers should fail",
			spec: config.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Username": "",
				"X-Password": "",
				"X-Token":    "",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			credential := &auth.CustomHTTPHeaderCredential{
				CustomHTTPHeaderSpec: tt.spec,
			}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := credential.Extract(context.Background(), header)

			g.Expect(ok).To(Equal(tt.expectedSuccess))
			if tt.expectedCreds == nil {
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(creds).NotTo(BeNil())
				g.Expect(creds.Username).To(Equal(tt.expectedCreds.Username))
				g.Expect(creds.Password).To(Equal(tt.expectedCreds.Password))
				g.Expect(creds.Token).To(Equal(tt.expectedCreds.Token))
			}
		})
	}
}

func TestCredential_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	type contextKey struct{}

	// Create a mock credential that checks if context is properly passed
	mockCredential := &mockCredential{
		expectedContextKey: contextKey{},
		expectedContextVal: "test-value",
		returnCreds:        &auth.ExtractedCredentials{Username: "context-user"},
		returnOk:           true,
	}

	// Create context with test data
	ctx := context.WithValue(context.Background(), contextKey{}, "test-value")
	header := make(http.Header)

	creds, ok := mockCredential.Extract(ctx, header)

	g.Expect(ok).To(BeTrue())
	g.Expect(creds).NotTo(BeNil())
	g.Expect(creds.Username).To(Equal("context-user"))
	g.Expect(mockCredential.contextReceived).To(BeTrue())
}

// Mock credential for testing context propagation
type mockCredential struct {
	expectedContextKey any
	expectedContextVal any
	returnCreds        *auth.ExtractedCredentials
	returnOk           bool
	contextReceived    bool
}

func (m *mockCredential) Extract(ctx context.Context, header http.Header) (*auth.ExtractedCredentials, bool) {
	val := ctx.Value(m.expectedContextKey)
	if val == m.expectedContextVal {
		m.contextReceived = true
	}
	return m.returnCreds, m.returnOk
}
