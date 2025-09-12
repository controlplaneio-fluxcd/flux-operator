// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// AuthenticatorFunc is a function that performs authentication.
type AuthenticatorFunc func(context.Context, http.Header) (context.Context, error)

// New creates a new authenticator function for the MCP server.
// The function extracts credentials using the provided credential
// and authenticates them using the provided provider. An authenticated
// context is returned upon successful authentication.
func New(credential Credential, provider Provider) AuthenticatorFunc {
	return func(ctx context.Context, header http.Header) (context.Context, error) {
		creds, ok := credential.Extract(ctx, header)
		if !ok {
			return nil, errors.New("failed to extract credentials from request")
		}
		sess, err := provider.Authenticate(ctx, *creds)
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate request: %w", err)
		}
		return context.WithValue(ctx, sessionContextKey{}, sess), nil
	}
}
