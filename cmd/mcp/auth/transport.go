// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"net/http"
	"strings"
)

// Transport extracts credentials from the incoming request.
type Transport interface {
	Extract(ctx context.Context, header http.Header) (*Credentials, bool)
}

// TransportSet is a set of available Transports.
// It implements the Transport interface by trying each Transport in order
// until one succeeds or all fail. If all fail, it returns false.
type TransportSet []Transport

// Extract implements Transport.
func (t TransportSet) Extract(ctx context.Context, header http.Header) (*Credentials, bool) {
	for _, transport := range t {
		creds, ok := transport.Extract(ctx, header)
		if ok {
			return creds, true
		}
	}
	return nil, false
}

// BearerTokenTransport is a transport that extracts credentials from the Authorization header,
// expecting a Bearer token.
type BearerTokenTransport struct{}

// Extract implements Transport.
func (BearerTokenTransport) Extract(ctx context.Context, header http.Header) (*Credentials, bool) {
	authz := header.Get("Authorization")
	if authz == "" {
		return nil, false
	}
	token := strings.TrimPrefix(authz, "Bearer ")
	if token == "" {
		return nil, false
	}
	return &Credentials{
		Token: token,
	}, true
}

// BasicAuthTransport is a transport that extracts credentials from the Authorization header,
// expecting Basic authentication.
type BasicAuthTransport struct{}

// Extract implements Transport.
func (BasicAuthTransport) Extract(ctx context.Context, header http.Header) (*Credentials, bool) {
	username, password, ok := (&http.Request{Header: header}).BasicAuth()
	if !ok {
		return nil, false
	}
	return &Credentials{
		Username: username,
		Password: password,
	}, true
}

// CustomHTTPHeaderTransport is a transport that extracts credentials from custom HTTP headers.
type CustomHTTPHeaderTransport struct{ CustomHTTPHeaderSpec }

// Extract implements Transport.
func (c *CustomHTTPHeaderTransport) Extract(ctx context.Context, header http.Header) (*Credentials, bool) {
	username := header.Get(c.Username)
	password := header.Get(c.Password)
	token := header.Get(c.Token)
	if username == "" && password == "" && token == "" {
		return nil, false
	}
	return &Credentials{
		Username: username,
		Password: password,
		Token:    token,
	}, true
}
