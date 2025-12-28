// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package user

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Details holds the user authentication details.
type Details struct {
	Profile
	Impersonation
}

// Profile holds the user profile information for display purposes.
type Profile struct {
	Name string
}

// Impersonation holds the user details for Kubernetes RBAC impersonation.
type Impersonation struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

// session holds the user session information during the life of a request.
type session struct {
	Details
	kubeClient any // We use the any type here because the kubeclient package needs to import this package.
}

// Key generates a unique key for the user based on username and groups.
func (u *session) Key() string {
	if u == nil {
		// There's a single user key when auth is not configured.
		return "privileged-user"
	}
	return Key(u.Impersonation)
}

// KubeClient returns the Kubernetes client associated with the session.
func (u *session) KubeClient() any {
	if u == nil {
		return nil
	}
	return u.kubeClient
}

// Key generates a unique key for the user based on username and groups.
func Key(imp Impersonation) string {
	var key strings.Builder
	fmt.Fprintf(&key, "username=%s", imp.Username)
	for _, group := range imp.Groups {
		fmt.Fprintf(&key, "\ngroup=%s", group)
	}
	return key.String()
}

// sessionContextKey is the context key for storing session values.
type sessionContextKey struct{}

// StoreSession stores the session in the given context.
func StoreSession(ctx context.Context, details Details, kubeClient any) context.Context {
	slices.Sort(details.Groups)
	return context.WithValue(ctx, sessionContextKey{}, &session{
		Details:    details,
		kubeClient: kubeClient,
	})
}

// LoadSession retrieves the session from the given context.
// If a non-nil session is found, authentication is configured.
// If nil, authentication is not configured.
func LoadSession(ctx context.Context) *session {
	if v := ctx.Value(sessionContextKey{}); v != nil {
		return v.(*session)
	}
	return nil
}

// KubeClient returns the Kubernetes client from the session in the context.
// If nil is returned, authentication is not configured.
func KubeClient(ctx context.Context) any {
	if s := LoadSession(ctx); s != nil {
		return s.kubeClient
	}
	return nil
}

// Permissions returns the user's impersonation details from the session in the context.
func Permissions(ctx context.Context) Impersonation {
	var imp Impersonation
	if s := LoadSession(ctx); s != nil {
		imp = s.Impersonation
	}
	return imp
}

// UsernameAndRole returns the username and role for display purposes.
func UsernameAndRole(ctx context.Context) (string, string) {
	s := LoadSession(ctx)
	hn := os.Getenv("HOSTNAME")

	switch {
	case s == nil && hn == "":
		// Authentication is not configured, and no pod hostname is set.
		// We are using a local kubeconfig in development mode.
		return "kubeconfig (dev)", ""
	case s == nil:
		// Authentication is not configured, but pod hostname is set.
		// We are using the pod's service account.
		return hn, ""
	case s.Name != "":
		// We are using an identity provider.
		// Then only name is relevant for display.
		return s.Name, ""
	default:
		// We are using an identity provider that does not provide
		// a name, or we are using Anonymous authentication.
		// Return username and role (groups).
		return s.Username, strings.Join(s.Groups, ", ")
	}
}
