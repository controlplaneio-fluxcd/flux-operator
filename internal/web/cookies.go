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
	cookieNameError            = "error"
	cookieNameAuthStorage      = "auth"
	cookieNameOAuth2LoginState = "state"

	cookiePathError            = "/"
	cookiePathAuthStorage      = "/"
	cookiePathOAuth2LoginState = "/oauth2/"

	cookieDurationShortLived = 5 * time.Minute
)

// setErrorCookie sets an error cookie in the response.
func setErrorCookie(w http.ResponseWriter, err error, code int) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieNameError,
		Path:     cookiePathError,
		Value:    fmt.Sprintf("HTTP %d: %v", code, err),
		Secure:   false,
		HttpOnly: false, // JavaScript should consume the cookie and clean it up.
		MaxAge:   int(cookieDurationShortLived.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// setAuthCookie sets an authentication cookie in the response.
func setAuthCookie(w http.ResponseWriter, name, path, value string, maxAge time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Path:     path,
		Value:    value,
		Secure:   secure,
		HttpOnly: true, // JavaScript should not touch auth cookies.
		MaxAge:   int(maxAge.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

// deleteAuthCookie deletes an authentication cookie in the response.
func deleteAuthCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		Path:   path,
		MaxAge: -1,
	})
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
	setAuthCookie(w, cookieNameAuthStorage, cookiePathAuthStorage, cValue,
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
	deleteAuthCookie(w, cookieNameAuthStorage, cookiePathAuthStorage)
}
