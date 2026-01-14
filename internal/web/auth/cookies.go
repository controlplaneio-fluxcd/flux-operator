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

	// Cookie chunking constants for handling large OIDC tokens.
	// Maximum safe size per cookie chunk (leaving room for cookie attributes).
	// Browser limit is 4KB but we use 3.5KB to leave room for cookie name, path,
	// domain, and other attributes that count toward the limit.
	cookieChunkMaxSize = 3584

	// Maximum number of chunks allowed (prevents abuse, covers tokens up to ~35KB).
	cookieChunkMaxCount = 10

	// Separator for chunk cookies: auth-storage-1, auth-storage-2, etc.
	cookieChunkSeparator = "-"
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
// It first clears any existing auth provider cookie to avoid duplicates.
func setAuthProviderCookie(w http.ResponseWriter, provider, loginURL string, authenticated bool) {
	clearCookieFromResponse(w, cookieNameAuthProvider)
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
// It first clears any existing auth storage cookies to avoid duplicates.
// For large tokens, the value is automatically split across multiple cookies.
func setAuthStorage(conf *config.ConfigSpec, w http.ResponseWriter, storage authStorage) {
	// Clear any existing auth storage cookies (including chunks).
	clearChunkedCookiesFromResponse(w, cookieNameAuthStorage)

	b, _ := json.Marshal(storage)
	cValue := base64.RawURLEncoding.EncodeToString(b)

	// Set chunked cookies (automatically handles single vs multiple cookies).
	_ = setChunkedCookies(w, cookieNameAuthStorage, cookiePathAuthStorage, cValue,
		conf.Authentication.SessionDuration.Duration, !conf.Insecure)
}

// getAuthStorage retrieves the authStorage from the request cookies.
// It automatically handles both single cookies and chunked cookies.
func getAuthStorage(r *http.Request) (*authStorage, error) {
	value, err := getChunkedCookieValue(r, cookieNameAuthStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth storage cookie: %w", err)
	}

	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode auth storage cookie: %w", err)
	}

	var storage authStorage
	if err := json.Unmarshal(b, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth storage cookie: %w", err)
	}

	return &storage, nil
}

// deleteAuthStorage deletes all authStorage cookies in the response,
// including any chunk cookies from previous large tokens.
func deleteAuthStorage(w http.ResponseWriter) {
	deleteChunkedCookies(w, cookieNameAuthStorage, cookiePathAuthStorage)
}

// clearCookieFromResponse removes a specific cookie from the response headers.
func clearCookieFromResponse(w http.ResponseWriter, name string) {
	cookies := w.Header().Values("Set-Cookie")
	w.Header().Del("Set-Cookie")
	for _, c := range cookies {
		if !strings.HasPrefix(c, name+"=") {
			w.Header().Add("Set-Cookie", c)
		}
	}
}

// splitIntoChunks splits a string value into chunks of the specified maximum size.
// Returns an error if the value would require more than the maximum allowed chunks.
func splitIntoChunks(value string, maxChunkSize, maxChunks int) ([]string, error) {
	if len(value) <= maxChunkSize {
		return []string{value}, nil
	}

	numChunks := (len(value) + maxChunkSize - 1) / maxChunkSize
	if numChunks > maxChunks {
		return nil, fmt.Errorf("value too large: requires %d chunks, maximum allowed is %d", numChunks, maxChunks)
	}

	chunks := make([]string, 0, numChunks)
	for i := 0; i < len(value); i += maxChunkSize {
		end := min(i+maxChunkSize, len(value))
		chunks = append(chunks, value[i:end])
	}
	return chunks, nil
}

// chunkCookieName returns the cookie name for a given chunk index.
// Index 0 returns the base name for backward compatibility with single-cookie storage.
func chunkCookieName(baseName string, index int) string {
	if index == 0 {
		return baseName
	}
	return fmt.Sprintf("%s%s%d", baseName, cookieChunkSeparator, index)
}

// getChunkedCookieValue retrieves and reassembles a potentially chunked cookie value.
// It first tries to read a single cookie (backward compatibility), then looks for chunks.
func getChunkedCookieValue(r *http.Request, baseName string) (string, error) {
	// First, try to get the base cookie.
	baseCookie, err := r.Cookie(baseName)
	if err != nil {
		return "", fmt.Errorf("failed to get cookie %s: %w", baseName, err)
	}

	// Check if this is a single-value cookie (no chunks exist)
	// by looking for the first chunk cookie.
	_, chunk1Err := r.Cookie(chunkCookieName(baseName, 1))
	if chunk1Err != nil {
		// No chunk-1 cookie means this is a single-value cookie (legacy or small value).
		return baseCookie.Value, nil
	}

	// Chunked cookie detected - reassemble all chunks.
	var builder strings.Builder
	builder.WriteString(baseCookie.Value)

	for i := 1; i < cookieChunkMaxCount; i++ {
		chunkCookie, err := r.Cookie(chunkCookieName(baseName, i))
		if err != nil {
			// No more chunks.
			break
		}
		builder.WriteString(chunkCookie.Value)
	}

	return builder.String(), nil
}

// setChunkedCookies sets a value across multiple cookies if needed.
// For values smaller than maxChunkSize, sets a single cookie.
// For larger values, splits across multiple cookies.
func setChunkedCookies(w http.ResponseWriter, baseName, path, value string,
	maxAge time.Duration, secure bool) error {

	chunks, err := splitIntoChunks(value, cookieChunkMaxSize, cookieChunkMaxCount)
	if err != nil {
		return err
	}

	for i, chunk := range chunks {
		name := chunkCookieName(baseName, i)
		setSecureCookie(w, name, path, chunk, maxAge, secure)
	}

	return nil
}

// deleteChunkedCookies deletes the base cookie and all potential chunk cookies.
func deleteChunkedCookies(w http.ResponseWriter, baseName, path string) {
	// Delete the base cookie.
	deleteCookie(w, baseName, path)

	// Delete all potential chunk cookies.
	for i := 1; i < cookieChunkMaxCount; i++ {
		deleteCookie(w, chunkCookieName(baseName, i), path)
	}
}

// clearChunkedCookiesFromResponse removes all chunk cookies for a base name
// from the response headers (pending Set-Cookie headers).
func clearChunkedCookiesFromResponse(w http.ResponseWriter, baseName string) {
	// Clear the base cookie.
	clearCookieFromResponse(w, baseName)

	// Clear all potential chunk cookies.
	for i := 1; i < cookieChunkMaxCount; i++ {
		clearCookieFromResponse(w, chunkCookieName(baseName, i))
	}
}
