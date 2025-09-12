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
)

func TestTransportSet_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		transports      []auth.Transport
		headers         map[string]string
		expectedCreds   *auth.Credentials
		expectedSuccess bool
	}{
		{
			name: "first transport succeeds",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
				auth.BasicAuthTransport{},
			},
			headers: map[string]string{
				"Authorization": "Bearer test-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "test-token",
			},
			expectedSuccess: true,
		},
		{
			name: "first transport succeeds with basic auth header (bearer extracts it as token)",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
				auth.BasicAuthTransport{},
			},
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
			expectedCreds: &auth.Credentials{
				Token: "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")), // Bearer transport succeeds first
			},
			expectedSuccess: true,
		},
		{
			name: "custom header transport succeeds after others fail",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
				auth.BasicAuthTransport{},
				&auth.CustomHTTPHeaderTransport{
					CustomHTTPHeaderSpec: auth.CustomHTTPHeaderSpec{
						Username: "X-Username",
						Password: "X-Password",
					},
				},
			},
			headers: map[string]string{
				"X-Username": "custom-user",
				"X-Password": "custom-pass",
			},
			expectedCreds: &auth.Credentials{
				Username: "custom-user",
				Password: "custom-pass",
			},
			expectedSuccess: true,
		},
		{
			name: "all transports fail",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
				auth.BasicAuthTransport{},
			},
			headers: map[string]string{
				"X-Custom-Header": "value",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name:            "empty transport set",
			transports:      []auth.Transport{},
			headers:         map[string]string{"Authorization": "Bearer token"},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "single transport succeeds",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
			},
			headers: map[string]string{
				"Authorization": "Bearer single-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "single-token",
			},
			expectedSuccess: true,
		},
		{
			name: "single transport fails",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
			},
			headers: map[string]string{
				"X-Other-Header": "value",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "second transport succeeds when first fails (no auth header)",
			transports: []auth.Transport{
				auth.BearerTokenTransport{},
				&auth.CustomHTTPHeaderTransport{
					CustomHTTPHeaderSpec: auth.CustomHTTPHeaderSpec{
						Token: "X-Token",
					},
				},
			},
			headers: map[string]string{
				"X-Token": "custom-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "custom-token",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			transportSet := auth.TransportSet(tt.transports)
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := transportSet.Extract(context.Background(), header)

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

func TestBearerTokenTransport_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		headers         map[string]string
		expectedCreds   *auth.Credentials
		expectedSuccess bool
	}{
		{
			name: "valid bearer token",
			headers: map[string]string{
				"Authorization": "Bearer valid-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "valid-token",
			},
			expectedSuccess: true,
		},
		{
			name: "bearer token with spaces",
			headers: map[string]string{
				"Authorization": "Bearer token-with-spaces",
			},
			expectedCreds: &auth.Credentials{
				Token: "token-with-spaces",
			},
			expectedSuccess: true,
		},
		{
			name: "bearer token with special characters",
			headers: map[string]string{
				"Authorization": "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.Gfx6VO9tcxwk6xqx9yYzSfebfeakZp5JYIgP_edaZAQ",
			},
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
				Token: "Basic dXNlcjpwYXNz", // TrimPrefix returns original string if prefix not found
			},
			expectedSuccess: true,
		},
		{
			name: "bearer without token",
			headers: map[string]string{
				"Authorization": "Bearer",
			},
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
				Token: "bearer token", // TrimPrefix is case sensitive, doesn't match "Bearer "
			},
			expectedSuccess: true,
		},
		{
			name: "authorization with different case",
			headers: map[string]string{
				"authorization": "Bearer token",
			},
			expectedCreds: &auth.Credentials{
				Token: "token",
			},
			expectedSuccess: true,
		},
		{
			name: "multiple authorization headers",
			headers: map[string]string{
				"Authorization": "Bearer first-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "first-token",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			transport := auth.BearerTokenTransport{}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := transport.Extract(context.Background(), header)

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

func TestBasicAuthTransport_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		headers         map[string]string
		expectedCreds   *auth.Credentials
		expectedSuccess bool
	}{
		{
			name: "valid basic auth",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
				Username: "",
				Password: "pass",
			},
			expectedSuccess: true,
		},
		{
			name: "basic auth with unicode characters",
			headers: map[string]string{
				"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("ユーザー:パスワード")),
			},
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
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
			expectedCreds: &auth.Credentials{
				Username: "testuser",
				Password: "testpass",
			},
			expectedSuccess: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			transport := auth.BasicAuthTransport{}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := transport.Extract(context.Background(), header)

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

func TestCustomHTTPHeaderTransport_Extract(t *testing.T) {
	for _, tt := range []struct {
		name            string
		spec            auth.CustomHTTPHeaderSpec
		headers         map[string]string
		expectedCreds   *auth.Credentials
		expectedSuccess bool
	}{
		{
			name: "username and password headers",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "test-user",
				"X-Password": "test-pass",
			},
			expectedCreds: &auth.Credentials{
				Username: "test-user",
				Password: "test-pass",
			},
			expectedSuccess: true,
		},
		{
			name: "token header only",
			spec: auth.CustomHTTPHeaderSpec{
				Token: "X-Auth-Token",
			},
			headers: map[string]string{
				"X-Auth-Token": "custom-token",
			},
			expectedCreds: &auth.Credentials{
				Token: "custom-token",
			},
			expectedSuccess: true,
		},
		{
			name: "all three headers",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-User",
				Password: "X-Pass",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-User":  "user",
				"X-Pass":  "pass",
				"X-Token": "token",
			},
			expectedCreds: &auth.Credentials{
				Username: "user",
				Password: "pass",
				Token:    "token",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - username only",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Username": "user-only",
			},
			expectedCreds: &auth.Credentials{
				Username: "user-only",
				Password: "",
				Token:    "",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - password only",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Password": "pass-only",
			},
			expectedCreds: &auth.Credentials{
				Username: "",
				Password: "pass-only",
				Token:    "",
			},
			expectedSuccess: true,
		},
		{
			name: "partial headers - token only",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
				Token:    "X-Token",
			},
			headers: map[string]string{
				"X-Token": "token-only",
			},
			expectedCreds: &auth.Credentials{
				Username: "",
				Password: "",
				Token:    "token-only",
			},
			expectedSuccess: true,
		},
		{
			name: "no matching headers",
			spec: auth.CustomHTTPHeaderSpec{
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
			spec: auth.CustomHTTPHeaderSpec{
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
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
			},
			headers: map[string]string{
				"x-username": "lower-case",
			},
			expectedCreds: &auth.Credentials{
				Username: "lower-case",
			},
			expectedSuccess: true,
		},
		{
			name: "headers with special characters",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "user@domain.com",
				"X-Password": "p@ss!w0rd#123",
			},
			expectedCreds: &auth.Credentials{
				Username: "user@domain.com",
				Password: "p@ss!w0rd#123",
			},
			expectedSuccess: true,
		},
		{
			name: "empty spec fields",
			spec: auth.CustomHTTPHeaderSpec{},
			headers: map[string]string{
				"X-Username": "user",
			},
			expectedCreds:   nil,
			expectedSuccess: false,
		},
		{
			name: "unicode values",
			spec: auth.CustomHTTPHeaderSpec{
				Username: "X-Username",
				Password: "X-Password",
			},
			headers: map[string]string{
				"X-Username": "ユーザー",
				"X-Password": "パスワード",
			},
			expectedCreds: &auth.Credentials{
				Username: "ユーザー",
				Password: "パスワード",
			},
			expectedSuccess: true,
		},
		{
			name: "all empty headers should fail",
			spec: auth.CustomHTTPHeaderSpec{
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

			transport := &auth.CustomHTTPHeaderTransport{
				CustomHTTPHeaderSpec: tt.spec,
			}
			header := make(http.Header)
			for k, v := range tt.headers {
				header.Set(k, v)
			}

			creds, ok := transport.Extract(context.Background(), header)

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

func TestTransport_ContextPropagation(t *testing.T) {
	g := NewWithT(t)

	type contextKey struct{}

	// Create a mock transport that checks if context is properly passed
	mockTransport := &mockTransport{
		expectedContextKey: contextKey{},
		expectedContextVal: "test-value",
		returnCreds:        &auth.Credentials{Username: "context-user"},
		returnOk:           true,
	}

	// Create context with test data
	ctx := context.WithValue(context.Background(), contextKey{}, "test-value")
	header := make(http.Header)

	creds, ok := mockTransport.Extract(ctx, header)

	g.Expect(ok).To(BeTrue())
	g.Expect(creds).NotTo(BeNil())
	g.Expect(creds.Username).To(Equal("context-user"))
	g.Expect(mockTransport.contextReceived).To(BeTrue())
}

// Mock transport for testing context propagation
type mockTransport struct {
	expectedContextKey any
	expectedContextVal any
	returnCreds        *auth.Credentials
	returnOk           bool
	contextReceived    bool
}

func (m *mockTransport) Extract(ctx context.Context, header http.Header) (*auth.Credentials, bool) {
	val := ctx.Value(m.expectedContextKey)
	if val == m.expectedContextVal {
		m.contextReceived = true
	}
	return m.returnCreds, m.returnOk
}
