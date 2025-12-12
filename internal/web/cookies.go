// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
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
func setAuthErrorCookie(w http.ResponseWriter, err error, code int) {
	setCookie(w, cookieNameAuthError, map[string]any{
		"code": code,
		"msg":  err.Error(),
	})
}

// setAuthProviderCookie sets the auth provider cookie in the response.
func setAuthProviderCookie(w http.ResponseWriter, provider, loginURL string, authenticated bool) {
	setCookie(w, cookieNameAuthProvider, map[string]any{
		"provider":      provider,
		"url":           loginURL,
		"authenticated": authenticated,
	})
}

// setAnonymousAuthProviderCookie sets the anonymous auth provider cookie in the response.
func setAnonymousAuthProviderCookie(w http.ResponseWriter) {
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
func setAuthStorage(conf *config.ConfigSpec, w http.ResponseWriter, storage authStorage) error {
	b, err := json.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal auth storage cookie: %w", err)
	}
	cValue := base64.RawURLEncoding.EncodeToString(b)
	setSecureCookie(w, cookieNameAuthStorage, cookiePathAuthStorage, cValue,
		conf.Authentication.SessionDuration.Duration, !conf.Insecure)
	return nil
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
