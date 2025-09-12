// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

// ExtractedCredentials represents authentication credentials extracted from the request.
type ExtractedCredentials struct {
	Username string
	Password string
	Token    string
}

// Credential extracts credentials from the incoming request.
type Credential interface {
	Extract(ctx context.Context, header http.Header) (*ExtractedCredentials, bool)
}

// CredentialSet is a set of available Credentials.
// It implements the Credential interface by trying each Credential in order
// until one succeeds or all fail. If all fail, it returns false.
type CredentialSet []Credential

// Extract implements Credential.
func (t CredentialSet) Extract(ctx context.Context, header http.Header) (*ExtractedCredentials, bool) {
	for _, credential := range t {
		creds, ok := credential.Extract(ctx, header)
		if ok {
			return creds, true
		}
	}
	return nil, false
}

// BearerTokenCredential is a credential that extracts credentials from the Authorization header,
// expecting a Bearer token.
type BearerTokenCredential struct{}

// Extract implements Credential.
func (BearerTokenCredential) Extract(ctx context.Context, header http.Header) (*ExtractedCredentials, bool) {
	authz := header.Get("Authorization")
	if authz == "" {
		return nil, false
	}
	token := strings.TrimPrefix(authz, "Bearer ")
	if token == "" {
		return nil, false
	}
	return &ExtractedCredentials{
		Token: token,
	}, true
}

// BasicAuthCredential is a credential that extracts credentials from the Authorization header,
// expecting Basic authentication.
type BasicAuthCredential struct{}

// Extract implements Credential.
func (BasicAuthCredential) Extract(ctx context.Context, header http.Header) (*ExtractedCredentials, bool) {
	username, password, ok := (&http.Request{Header: header}).BasicAuth()
	if !ok {
		return nil, false
	}
	if username == "" && password == "" {
		return nil, false
	}
	return &ExtractedCredentials{
		Username: username,
		Password: password,
	}, true
}

// CustomHTTPHeaderCredential is a credential that extracts credentials from custom HTTP headers.
type CustomHTTPHeaderCredential struct{ config.CustomHTTPHeaderSpec }

// Extract implements Credential.
func (c *CustomHTTPHeaderCredential) Extract(ctx context.Context, header http.Header) (*ExtractedCredentials, bool) {
	username := header.Get(c.Username)
	password := header.Get(c.Password)
	token := header.Get(c.Token)
	if username == "" && password == "" && token == "" {
		return nil, false
	}
	return &ExtractedCredentials{
		Username: username,
		Password: password,
		Token:    token,
	}, true
}
