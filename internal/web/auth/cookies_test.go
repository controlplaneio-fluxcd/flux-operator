// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

func TestSetCookie(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()
	testData := map[string]any{"key": "value", "number": 42}
	setCookie(rec, "test-cookie", testData)

	cookies := rec.Result().Cookies()
	g.Expect(cookies).To(HaveLen(1))

	cookie := cookies[0]
	g.Expect(cookie.Name).To(Equal("test-cookie"))
	g.Expect(cookie.Path).To(Equal("/"))
	g.Expect(cookie.SameSite).To(Equal(http.SameSiteLaxMode))

	// Decode and verify value
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	g.Expect(err).NotTo(HaveOccurred())

	var result map[string]any
	err = json.Unmarshal(decoded, &result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["key"]).To(Equal("value"))
	g.Expect(result["number"]).To(BeNumerically("==", 42))
}

func TestSetSecureCookie(t *testing.T) {
	for _, tt := range []struct {
		name       string
		secure     bool
		duration   time.Duration
		wantSecure bool
	}{
		{
			name:       "secure cookie when not insecure",
			secure:     true,
			duration:   time.Hour,
			wantSecure: true,
		},
		{
			name:       "insecure cookie when insecure mode",
			secure:     false,
			duration:   time.Hour,
			wantSecure: false,
		},
		{
			name:       "correct max age from duration",
			secure:     true,
			duration:   30 * time.Minute,
			wantSecure: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			rec := httptest.NewRecorder()
			setSecureCookie(rec, "secure-cookie", "/test", "test-value", tt.duration, tt.secure)

			cookies := rec.Result().Cookies()
			g.Expect(cookies).To(HaveLen(1))

			cookie := cookies[0]
			g.Expect(cookie.Name).To(Equal("secure-cookie"))
			g.Expect(cookie.Path).To(Equal("/test"))
			g.Expect(cookie.Value).To(Equal("test-value"))
			g.Expect(cookie.Secure).To(Equal(tt.wantSecure))
			g.Expect(cookie.HttpOnly).To(BeTrue())
			g.Expect(cookie.MaxAge).To(Equal(int(tt.duration.Seconds())))
			g.Expect(cookie.SameSite).To(Equal(http.SameSiteLaxMode))
		})
	}
}

func TestDeleteCookie(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()
	deleteCookie(rec, "delete-me", "/path")

	cookies := rec.Result().Cookies()
	g.Expect(cookies).To(HaveLen(1))

	cookie := cookies[0]
	g.Expect(cookie.Name).To(Equal("delete-me"))
	g.Expect(cookie.Path).To(Equal("/path"))
	g.Expect(cookie.MaxAge).To(Equal(-1))
}

func TestSetAuthErrorCookie(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()
	setAuthErrorCookie(rec, errInternalError)

	cookies := rec.Result().Cookies()
	g.Expect(cookies).To(HaveLen(1))

	cookie := cookies[0]
	g.Expect(cookie.Name).To(Equal(cookieNameAuthError))

	// Decode and verify sanitized message
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	g.Expect(err).NotTo(HaveOccurred())

	var result map[string]any
	err = json.Unmarshal(decoded, &result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["msg"]).To(ContainSubstring("internal error"))
}

func TestSetAuthProviderCookie(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()
	setAuthProviderCookie(rec, "OIDC", "/oauth2/authorize", true)

	cookies := rec.Result().Cookies()
	g.Expect(cookies).To(HaveLen(1))

	cookie := cookies[0]
	g.Expect(cookie.Name).To(Equal(cookieNameAuthProvider))

	// Decode and verify structure
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	g.Expect(err).NotTo(HaveOccurred())

	var result map[string]any
	err = json.Unmarshal(decoded, &result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["provider"]).To(Equal("OIDC"))
	g.Expect(result["url"]).To(Equal("/oauth2/authorize"))
	g.Expect(result["authenticated"]).To(BeTrue())
}

func TestSetAuthProviderCookie_ClearsExisting(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()

	// Set first cookie
	setAuthProviderCookie(rec, "First", "/first", false)

	// Set second cookie (should clear first)
	setAuthProviderCookie(rec, "Second", "/second", true)

	cookies := rec.Result().Cookies()
	// Should only have one auth-provider cookie
	authProviderCookies := 0
	for _, c := range cookies {
		if c.Name == cookieNameAuthProvider {
			authProviderCookies++
		}
	}
	g.Expect(authProviderCookies).To(Equal(1))

	// Verify it's the second one
	for _, c := range cookies {
		if c.Name == cookieNameAuthProvider {
			decoded, _ := base64.RawURLEncoding.DecodeString(c.Value)
			var result map[string]any
			_ = json.Unmarshal(decoded, &result)
			g.Expect(result["provider"]).To(Equal("Second"))
		}
	}
}

func TestSetAnonymousAuthProviderCookie(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()
	SetAnonymousAuthProviderCookie(rec)

	cookies := rec.Result().Cookies()
	g.Expect(cookies).To(HaveLen(1))

	cookie := cookies[0]
	g.Expect(cookie.Name).To(Equal(cookieNameAuthProvider))

	// Decode and verify Anonymous provider with authenticated=true
	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	g.Expect(err).NotTo(HaveOccurred())

	var result map[string]any
	err = json.Unmarshal(decoded, &result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["provider"]).To(Equal(config.AuthenticationTypeAnonymous))
	g.Expect(result["url"]).To(BeEmpty())
	g.Expect(result["authenticated"]).To(BeTrue())
}

func TestAuthStorage(t *testing.T) {
	t.Run("setAuthStorage stores tokens", func(t *testing.T) {
		g := NewWithT(t)

		conf := &config.ConfigSpec{
			Insecure: false,
			Authentication: &config.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}
		rec := httptest.NewRecorder()
		storage := authStorage{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
		}
		setAuthStorage(conf, rec, storage)

		cookies := rec.Result().Cookies()
		g.Expect(cookies).To(HaveLen(1))

		cookie := cookies[0]
		g.Expect(cookie.Name).To(Equal(cookieNameAuthStorage))
		g.Expect(cookie.Path).To(Equal(cookiePathAuthStorage))
		g.Expect(cookie.Secure).To(BeTrue())
		g.Expect(cookie.HttpOnly).To(BeTrue())
	})

	t.Run("getAuthStorage retrieves tokens", func(t *testing.T) {
		g := NewWithT(t)

		storage := authStorage{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
		}
		b, _ := json.Marshal(storage)
		encoded := base64.RawURLEncoding.EncodeToString(b)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  cookieNameAuthStorage,
			Value: encoded,
		})

		result, err := getAuthStorage(req)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.AccessToken).To(Equal("access-token-123"))
		g.Expect(result.RefreshToken).To(Equal("refresh-token-456"))
	})

	t.Run("getAuthStorage returns error for missing cookie", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := getAuthStorage(req)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to get auth storage cookie"))
	})

	t.Run("getAuthStorage returns error for invalid base64", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  cookieNameAuthStorage,
			Value: "not-valid-base64!!!",
		})

		_, err := getAuthStorage(req)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to decode auth storage cookie"))
	})

	t.Run("getAuthStorage returns error for invalid JSON", func(t *testing.T) {
		g := NewWithT(t)

		encoded := base64.RawURLEncoding.EncodeToString([]byte("not json"))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{
			Name:  cookieNameAuthStorage,
			Value: encoded,
		})

		_, err := getAuthStorage(req)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to unmarshal auth storage cookie"))
	})

	t.Run("deleteAuthStorage deletes the cookie", func(t *testing.T) {
		g := NewWithT(t)

		rec := httptest.NewRecorder()
		deleteAuthStorage(rec)

		cookies := rec.Result().Cookies()
		g.Expect(cookies).To(HaveLen(1))

		cookie := cookies[0]
		g.Expect(cookie.Name).To(Equal(cookieNameAuthStorage))
		g.Expect(cookie.MaxAge).To(Equal(-1))
	})
}

func TestClearCookieFromResponse(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()

	// Set multiple cookies
	http.SetCookie(rec, &http.Cookie{Name: "keep-me", Value: "value1"})
	http.SetCookie(rec, &http.Cookie{Name: "remove-me", Value: "value2"})
	http.SetCookie(rec, &http.Cookie{Name: "also-keep", Value: "value3"})

	// Clear one specific cookie
	clearCookieFromResponse(rec, "remove-me")

	// Verify remaining cookies
	cookieHeaders := rec.Header().Values("Set-Cookie")
	g.Expect(cookieHeaders).To(HaveLen(2))

	hasKeepMe := false
	hasAlsoKeep := false
	hasRemoveMe := false
	for _, h := range cookieHeaders {
		if len(h) >= 7 && h[:7] == "keep-me" {
			hasKeepMe = true
		}
		if len(h) >= 9 && h[:9] == "also-keep" {
			hasAlsoKeep = true
		}
		if len(h) >= 9 && h[:9] == "remove-me" {
			hasRemoveMe = true
		}
	}
	g.Expect(hasKeepMe).To(BeTrue())
	g.Expect(hasAlsoKeep).To(BeTrue())
	g.Expect(hasRemoveMe).To(BeFalse())
}
