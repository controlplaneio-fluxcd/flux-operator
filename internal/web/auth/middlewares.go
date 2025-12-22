// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/kubeclient"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	authPathLogout      = "/logout"
	oauth2PathAuthorize = "/oauth2/authorize"
	oauth2PathCallback  = "/oauth2/callback"

	authQueryParamOriginalPath = "originalPath"
)

// NewMiddleware creates a new authentication middleware for HTTP handlers.
func NewMiddleware(conf *config.ConfigSpec, kubeClient *kubeclient.Client,
	initLog logr.Logger) (func(next http.Handler) http.Handler, error) {

	// Build middleware according to the authentication type.
	var middleware func(next http.Handler) http.Handler
	var provider string
	switch {
	case conf.Authentication == nil:
		middleware = newDefaultMiddleware()
		provider = "None"
	case conf.Authentication.Anonymous != nil:
		var err error
		middleware, err = newAnonymousMiddleware(conf, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create anonymous authentication middleware: %w", err)
		}
		provider = config.AuthenticationTypeAnonymous
	case conf.Authentication.OAuth2 != nil:
		var err error
		middleware, err = newOAuth2Middleware(conf, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("failed to create OAuth2 authentication middleware: %w", err)
		}
		provider = fmt.Sprintf("%s/%s", config.AuthenticationTypeOAuth2, conf.Authentication.OAuth2.Provider)
	default:
		return nil, fmt.Errorf("unsupported authentication method")
	}
	initLog.Info("authentication initialized successfully", "authProvider", provider)

	// Enhance middleware with logout handling and logger.
	return func(next http.Handler) http.Handler {
		next = middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case authPathLogout:
				// Only allow POST for logout to prevent CSRF attacks.
				// GET requests to /logout could be triggered by malicious links.
				if r.Method != http.MethodPost {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}
				deleteAuthStorage(w)
				http.Redirect(w, r, "/", http.StatusSeeOther)
			default:
				// Inject logger into context.
				l := log.FromContext(r.Context()).WithValues("authProvider", provider)
				ctx := log.IntoContext(r.Context(), l)
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
			}
		})
	}, nil
}

// newDefaultMiddleware creates a default authentication middleware
// that allows all requests without authentication.
func newDefaultMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SetAnonymousAuthProviderCookie(w)
			next.ServeHTTP(w, r)
		})
	}
}

// newAnonymousMiddleware creates an anonymous authentication middleware.
func newAnonymousMiddleware(conf *config.ConfigSpec, kubeClient *kubeclient.Client) (func(next http.Handler) http.Handler, error) {
	anonConf := conf.Authentication.Anonymous

	username := anonConf.Username
	groups := anonConf.Groups

	details := user.Details{
		Impersonation: user.Impersonation{
			Username: username,
			Groups:   groups,
		},
	}

	client, err := kubeClient.GetUserClientFromCache(details.Impersonation)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			SetAnonymousAuthProviderCookie(w)
			ctx := user.StoreSession(r.Context(), details, client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// newOAuth2Middleware creates an OAuth2 authentication middleware.
func newOAuth2Middleware(conf *config.ConfigSpec, kubeClient *kubeclient.Client) (func(next http.Handler) http.Handler, error) {
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
	authenticator, err := newOAuth2Authenticator(conf, kubeClient, provider)
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
			case strings.HasPrefix(r.URL.Path, "/api/"):
				authenticator.serveAPI(w, r, next)
			case r.URL.Path == "/":
				authenticator.serveIndex(w, r, next)
			default:
				next.ServeHTTP(w, r)
			}
		})
	}, nil
}
