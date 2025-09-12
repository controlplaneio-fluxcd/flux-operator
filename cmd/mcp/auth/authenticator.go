// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"errors"
	"fmt"
)

// Authenticator defines the interface for authenticating users based on provided credentials.
type Authenticator interface {
	Authenticate(ctx context.Context, credentials Credentials) (*Session, error)
}

// AuthenticatorSet is a set of available Authenticators.
// It implements the Authenticator interface by trying each Authenticator in order
// until one succeeds or all fail. If all fail, it returns an error containing the
// errors from each Authenticator.
type AuthenticatorSet []Authenticator

// Authenticate implements Authenticator.
func (a AuthenticatorSet) Authenticate(ctx context.Context, credentials Credentials) (*Session, error) {
	errs := make([]error, 0, len(a))
	for _, authenticator := range a {
		sess, err := authenticator.Authenticate(ctx, credentials)
		if err == nil {
			return sess, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("no authenticators succeeded: %w", errors.Join(errs...))
}
