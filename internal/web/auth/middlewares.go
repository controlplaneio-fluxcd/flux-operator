// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
)

const (
	authPathLogout      = "/logout"
	oauth2PathAuthorize = "/oauth2/authorize"
	oauth2PathCallback  = "/oauth2/callback"

	authQueryParamOriginalPath = "originalPath"
)

// NewMiddleware creates a new authentication middleware for HTTP handlers.
func NewMiddleware(ctx context.Context, conf *config.ConfigSpec, kubeClient *kubeclient.Client) (func(next http.Handler) http.Handler, error) {
	// Build middleware according to the authentication type.
	var middleware func(next http.Handler) http.Handler
	switch {
	case conf.Authentication == nil:
		middleware = newDefaultMiddleware()
	case conf.Authentication.Anonymous != nil:
		var err error
		middleware, err = newAnonymousMiddleware(conf, kubeClient)
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
			http.Redirect(w, r, "/", http.StatusSeeOther)
		})
	}, nil
}

// newDefaultMiddleware creates a default authentication middleware
// that allows all requests without authentication.
func newDefaultMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setAnonymousAuthProviderCookie(w)
			next.ServeHTTP(w, r)
		})
	}
}

// newAnonymousMiddleware creates an anonymous authentication middleware.
func newAnonymousMiddleware(conf *config.ConfigSpec, kubeClient *kubeclient.Client) (func(next http.Handler) http.Handler, error) {
	anonConf := conf.Authentication.Anonymous

	username := anonConf.Username
	groups := anonConf.Groups

	client, err := kubeClient.GetUserClientFromCache(username, groups)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setAnonymousAuthProviderCookie(w)
			ctx := kubeclient.StoreUserSession(r.Context(), username, groups, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// newOAuth2Middleware creates an OAuth2 authentication middleware.
func newOAuth2Middleware(ctx context.Context, conf *config.ConfigSpec, kubeClient *kubeclient.Client) (func(next http.Handler) http.Handler, error) {
	// Build the OAuth2 provider.
	var provider oauth2Provider
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
			switch {
			case r.URL.Path == oauth2PathAuthorize:
				authenticator.serveAuthorize(w, r)
			case r.URL.Path == oauth2PathCallback:
				authenticator.serveCallback(w, r)
			case isAPIRequest(r):
				authenticator.serveAPI(w, r, next)
			default:
				authenticator.serveAssets(w, r, next)
			}
		})
	}, nil
}

// respondAuthError responds to the client with an auth error message.
// For API requests, it responds with a plain error message and the
// given HTTP status code. For page requests, it stores an error cookie
// and redirects to /.
func respondAuthError(w http.ResponseWriter, r *http.Request, err error, code int) {
	switch {
	case isAPIRequest(r):
		http.Error(w, err.Error(), code)
	default:
		setAuthErrorCookie(w, err, code)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// isAPIRequest returns true if the request is for an API endpoint.
func isAPIRequest(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}
