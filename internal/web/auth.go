// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

const (
	authPathLogin      = "/login"
	authPathLogout     = "/logout"
	authPathError      = "/auth/error"
	oauth2PathCallback = "/oauth2/callback"

	authQueryParamOriginalPath = "originalPath"
)

// NewAuthMiddleware creates a new authentication middleware for HTTP handlers.
func NewAuthMiddleware(ctx context.Context, conf *config.ConfigSpec, kubeClient *Client) (func(next http.Handler) http.Handler, error) {
	// Build middleware according to the authentication type.
	var middleware func(next http.Handler) http.Handler
	switch {
	case conf.Authentication == nil:
		middleware = newDefaultAuthMiddleware()
	case conf.Authentication.Anonymous != nil:
		var err error
		middleware, err = newAnonymousAuthMiddleware(conf, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create anonymous authentication middleware: %w", err)
		}
	case conf.Authentication.OAuth2 != nil:
		var err error
		middleware, err = newOAuth2Middleware(ctx, conf, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create OAuth2 authentication middleware: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported authentication method")
	}

	// Enhance middleware with logout handling.
	return func(next http.Handler) http.Handler {
		next = middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != authPathLogout {
				next.ServeHTTP(w, r)
				return
			}
			deleteAuthStorage(w)
			http.Redirect(w, r, authPathLogin, http.StatusSeeOther)
		})
	}, nil
}

// newDefaultAuthMiddleware creates a default authentication middleware that
// allows all requests without authentication.
func newDefaultAuthMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == authPathLogin {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newAnonymousAuthMiddleware creates an anonymous authentication middleware.
func newAnonymousAuthMiddleware(conf *config.ConfigSpec, kubeClient *Client) (func(next http.Handler) http.Handler, error) {
	anonConf := conf.Authentication.Anonymous

	username := anonConf.Username
	groups := anonConf.Groups

	client, err := kubeClient.getUserClientFromCache(username, groups)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == authPathLogin {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			ctx := storeUserSession(r.Context(), config.AuthenticationTypeAnonymous, username, groups, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// newOAuth2Middleware creates an OAuth2 authentication middleware.
func newOAuth2Middleware(ctx context.Context, conf *config.ConfigSpec, kubeClient *Client) (func(next http.Handler) http.Handler, error) {
	// Build the OAuth2 provider.
	var provider *oauth2Provider
	var err error
	switch conf.Authentication.OAuth2.Provider {
	case config.OAuth2ProviderOIDC:
		provider, err = newOIDCProvider(conf)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported OAuth2 provider: %s", conf.Authentication.OAuth2.Provider)
	}

	// Build the OAuth2 authenticator.
	authenticator, err := newOAuth2Authenticator(ctx, conf, kubeClient, provider)
	if err != nil {
		return nil, err
	}

	// Return the middleware.
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case authPathLogin:
				authenticator.ServeLogin(w, r)
			case oauth2PathCallback:
				authenticator.ServeCallback(w, r)
			case authPathError:
				next.ServeHTTP(w, r)
			default:
				authenticator.ServeProtectedResource(w, r, next)
			}
		})
	}, nil
}

// respondAuthExpired responds to the client that authentication has expired.
// For API requests, it responds with a 401 Unauthorized status code.
// For page requests, it redirects to the /login page.
func respondAuthExpired(w http.ResponseWriter, r *http.Request, err error, qs ...url.Values) {
	if isAPIRequest(r) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// For page requests, build the redirect URL with query and original path
	// and redirect to /login.
	if len(qs) == 0 {
		q := r.URL.Query()
		q.Set(authQueryParamOriginalPath, r.URL.Path)
		qs = append(qs, q)
	}
	redirectURL := fmt.Sprintf("%s?%s", authPathLogin, qs[0].Encode())
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// respondAuthError responds to the client with a terminal error message.
// Useful for scenarios where retrying authentication automatically will not
// help, and it's better to inform the user about the problem. For API requests,
// it responds with a plain error message and the given HTTP status code.
// For page requests, it stores an error cookie and redirects to the
// /auth/error page.
func respondAuthError(w http.ResponseWriter, r *http.Request, err error, code int) {
	switch {
	case isAPIRequest(r):
		http.Error(w, err.Error(), code)
	default:
		setErrorCookie(w, err, code)
		http.Redirect(w, r, authPathError, http.StatusSeeOther)
	}
}
