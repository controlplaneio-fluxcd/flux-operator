// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"fmt"
	"strings"
)

// Session represents a user session in the MCP authentication system.
// It holds the username and groups for Kubernetes impersonation, and
// the scopes granted by the user to the token.
type Session struct {
	UserName string
	Groups   []string
	Scopes   []string
}

type sessionContextKey struct{}

// FromContext retrieves the Session from the given context.
func FromContext(ctx context.Context) *Session {
	v := ctx.Value(sessionContextKey{})
	if v == nil {
		return nil
	}
	return v.(*Session)
}

// IntoContext adds the Session to the given context.
func IntoContext(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// CheckScopes returns an error if the session does not include
// at least one of the required scopes. If authentication or
// scope validation are disabled, nil is returned.
func CheckScopes(ctx context.Context, requiredScopes []string) error {
	if len(requiredScopes) == 0 {
		return nil // No scopes required.
	}

	s := FromContext(ctx)
	if s == nil {
		// No session found, authentication is disabled.
		return nil
	}
	if s.Scopes == nil {
		// Scopes are nil, validation is disabled.
		return nil
	}

	scopes := make(map[string]struct{}, len(s.Scopes))
	for _, scope := range s.Scopes {
		scopes[scope] = struct{}{}
	}
	for _, reqScope := range requiredScopes {
		if _, ok := scopes[reqScope]; ok {
			return nil
		}
	}
	return fmt.Errorf("at least one of the following scopes is required: {%s}",
		strings.Join(requiredScopes, ", "))
}
