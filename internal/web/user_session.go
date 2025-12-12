// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"slices"
)

// userSession holds the user session information during the life of a request.
type userSession struct {
	username string
	groups   []string
	client   *userClient
}

// getUserKey generates a unique key for the user based on username and groups.
func (u *userSession) getUserKey() string {
	return getUserKey(u.username, u.groups)
}

// userSessionContextKey is the context key for storing userSession values.
type userSessionContextKey struct{}

// storeUserSession stores the userSession in the given context.
func storeUserSession(ctx context.Context, username string, groups []string, client *userClient) context.Context {
	if groups == nil {
		groups = []string{}
	}
	return context.WithValue(ctx, userSessionContextKey{}, &userSession{
		username: username,
		groups:   groups,
		client:   client,
	})
}

// loadUserSession retrieves the userSession from the given context.
// If a non-nil userSession is found, authentication is configured.
// If nil, authentication is not configured.
func loadUserSession(ctx context.Context) *userSession {
	if v := ctx.Value(userSessionContextKey{}); v != nil {
		return v.(*userSession)
	}
	return nil
}

// getUserKey generates a unique key for the user based on username and groups.
func getUserKey(username string, groups []string) string {
	key := fmt.Sprintf("username=%s", username)
	slices.Sort(groups)
	for _, group := range groups {
		key += fmt.Sprintf("\ngroup=%s", group)
	}
	return key
}
