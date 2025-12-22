// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

const (
	cookieNameAuthError        = "auth-error"
	cookieNameAuthProvider     = "auth-provider"
	cookieNameAuthStorage      = "auth-storage"
	cookieNameOAuth2LoginState = "oauth2-state"

	cookiePathAuthStorage      = "/"
	cookiePathOAuth2LoginState = "/oauth2/"

	cookieDurationShortLived = 5 * time.Minute
)

// setCookie sets a cookie in the response.
func setCookie(w http.ResponseWriter, name string, obj any) {
	b, _ := json.Marshal(obj)
	value := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     "/",
		Value:    value,
		SameSite: http.SameSiteLaxMode,
	})
}

// setSecureCookie sets a secure cookie in the response.
func setSecureCookie(w http.ResponseWriter, name, path, value string, maxAge time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     path,
		Value:    value,
		Secure:   secure,
		HttpOnly: true,
		MaxAge:   int(maxAge.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// deleteCookie deletes a cookie in the response.
func deleteCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		Path:   path,
		MaxAge: -1,
	})
}

// setAuthErrorCookie sets the auth error cookie in the response.
// Error messages are sanitized to avoid leaking internal details.
func setAuthErrorCookie(w http.ResponseWriter, err error) {
	setCookie(w, cookieNameAuthError, map[string]any{
		"msg": sanitizeErrorMessage(err),
	})
}

// setAuthProviderCookie sets the auth provider cookie in the response.
// It removes any previous auth provider cookies to avoid mistakes.
func setAuthProviderCookie(w http.ResponseWriter, provider, loginURL string, authenticated bool) {
	cookies := w.Header().Values("Set-Cookie")
	w.Header().Del("Set-Cookie")
	for _, c := range cookies {
		if !strings.HasPrefix(c, cookieNameAuthProvider+"=") {
			w.Header().Add("Set-Cookie", c)
		}
	}
	setCookie(w, cookieNameAuthProvider, map[string]any{
		"provider":      provider,
		"url":           loginURL,
		"authenticated": authenticated,
	})
}

// SetAnonymousAuthProviderCookie sets the anonymous auth provider cookie in the response.
func SetAnonymousAuthProviderCookie(w http.ResponseWriter) {
	setAuthProviderCookie(w, config.AuthenticationTypeAnonymous, "", true)
}

// authStorage holds the authentication information stored in cookies.
// The authentication information is the cryptographic material that
// is persisted after a successful authentication flow, and is used
// to authenticate subsequent API requests.
type authStorage struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// setAuthStorage sets the authStorage in the response cookies.
func setAuthStorage(conf *config.ConfigSpec, w http.ResponseWriter, storage authStorage) {
	b, _ := json.Marshal(storage)
	cValue := base64.RawURLEncoding.EncodeToString(b)
	setSecureCookie(w, cookieNameAuthStorage, cookiePathAuthStorage, cValue,
		conf.Authentication.SessionDuration.Duration, !conf.Insecure)
}

// getAuthStorage retrieves the authStorage from the request cookies.
func getAuthStorage(r *http.Request) (*authStorage, error) {
	c, err := r.Cookie(cookieNameAuthStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth storage cookie: %w", err)
	}
	b, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode auth storage cookie: %w", err)
	}
	var storage authStorage
	if err := json.Unmarshal(b, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth storage cookie: %w", err)
	}
	return &storage, nil
}

// deleteAuthStorage deletes the authStorage cookie in the response.
func deleteAuthStorage(w http.ResponseWriter) {
	deleteCookie(w, cookieNameAuthStorage, cookiePathAuthStorage)
}
