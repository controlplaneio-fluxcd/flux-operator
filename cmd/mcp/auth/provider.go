// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"errors"
	"fmt"
)

// Provider defines the interface for authenticating users based on provided credentials.
type Provider interface {
	Authenticate(ctx context.Context, credentials ExtractedCredentials) (*Session, error)
}

// ProviderSet is a set of available Providers.
// It implements the Provider interface by trying each Provider in order
// until one succeeds or all fail. If all fail, it returns an error containing the
// errors from each Provider.
type ProviderSet []Provider

// Authenticate implements Provider.
func (a ProviderSet) Authenticate(ctx context.Context, credentials ExtractedCredentials) (*Session, error) {
	errs := make([]error, 0, len(a))
	for _, provider := range a {
		sess, err := provider.Authenticate(ctx, credentials)
		if err == nil {
			return sess, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("no providers succeeded: %w", errors.Join(errs...))
}
