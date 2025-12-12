// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeclient

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// userSession holds the user session information during the life of a request.
type userSession struct {
	username string
	groups   []string
	client   *userClient
}

// getUserKey generates a unique key for the user based on username and groups.
func (u *userSession) getUserKey() string {
	if u == nil {
		// There's a single user key when auth is not configured.
		return "priviledged-user"
	}
	return getUserKey(u.username, u.groups)
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

// userSessionContextKey is the context key for storing userSession values.
type userSessionContextKey struct{}

// StoreUserSession stores the userSession in the given context.
func StoreUserSession(ctx context.Context, username string, groups []string, client *userClient) context.Context {
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

// UsernameAndRole returns the username and role of the user session.
func UsernameAndRole(ctx context.Context) (string, string) {
	if us := loadUserSession(ctx); us != nil {
		return us.username, strings.Join(us.groups, ", ")
	}
	return os.Getenv("HOSTNAME"), "cluster:view"
}
