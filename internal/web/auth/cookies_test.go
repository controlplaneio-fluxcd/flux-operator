// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
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
	g.Expect(result["provider"]).To(Equal(fluxcdv1.AuthenticationTypeAnonymous))
	g.Expect(result["url"]).To(BeEmpty())
	g.Expect(result["authenticated"]).To(BeTrue())
}

func TestAuthStorage(t *testing.T) {
	t.Run("setAuthStorage stores tokens", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Insecure: false,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}
		rec := httptest.NewRecorder()
		storage := authStorage{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
		}
		g.Expect(setAuthStorage(conf, rec, storage)).To(Succeed())

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

	t.Run("setAuthStorage and getAuthStorage preserve SessionStart", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Insecure: false,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}
		rec := httptest.NewRecorder()
		sessionStartTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		storage := authStorage{
			AccessToken:  "access-token-123",
			RefreshToken: "refresh-token-456",
			SessionStart: sessionStartTime,
		}
		g.Expect(setAuthStorage(conf, rec, storage)).To(Succeed())

		// Create a request with the cookies from the response
		cookies := rec.Result().Cookies()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		// Verify SessionStart is preserved
		result, err := getAuthStorage(req)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.AccessToken).To(Equal("access-token-123"))
		g.Expect(result.RefreshToken).To(Equal("refresh-token-456"))
		g.Expect(result.SessionStart).To(Equal(sessionStartTime))
	})

	t.Run("getAuthStorage handles zero SessionStart for backward compatibility", func(t *testing.T) {
		g := NewWithT(t)

		// Simulate an old storage without SessionStart (or with zero value)
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
		g.Expect(result.SessionStart.IsZero()).To(BeTrue())
	})

	t.Run("deleteAuthStorage deletes all chunk cookies", func(t *testing.T) {
		g := NewWithT(t)

		rec := httptest.NewRecorder()
		deleteAuthStorage(rec)

		cookies := rec.Result().Cookies()
		// Should have deletion cookies for base and all potential chunks.
		g.Expect(cookies).To(HaveLen(cookieChunkMaxCount))

		// Verify the base cookie is deleted.
		g.Expect(cookies[0].Name).To(Equal(cookieNameAuthStorage))
		g.Expect(cookies[0].MaxAge).To(Equal(-1))

		// Verify all chunk cookies are deleted.
		for i := 1; i < cookieChunkMaxCount; i++ {
			g.Expect(cookies[i].Name).To(Equal(chunkCookieName(cookieNameAuthStorage, i)))
			g.Expect(cookies[i].MaxAge).To(Equal(-1))
		}
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

func TestSplitIntoChunks(t *testing.T) {
	for _, tt := range []struct {
		name      string
		value     string
		maxSize   int
		maxChunks int
		wantCount int
		wantErr   bool
	}{
		{
			name:      "small value fits in single chunk",
			value:     "small",
			maxSize:   100,
			maxChunks: 10,
			wantCount: 1,
		},
		{
			name:      "value exactly at chunk size",
			value:     "abc",
			maxSize:   3,
			maxChunks: 10,
			wantCount: 1,
		},
		{
			name:      "value needs two chunks",
			value:     "abcdef",
			maxSize:   3,
			maxChunks: 10,
			wantCount: 2,
		},
		{
			name:      "value needs three chunks with remainder",
			value:     "abcdefgh",
			maxSize:   3,
			maxChunks: 10,
			wantCount: 3,
		},
		{
			name:      "exceeds max chunks",
			value:     "abcdefghij",
			maxSize:   3,
			maxChunks: 2,
			wantErr:   true,
		},
		{
			name:      "empty value",
			value:     "",
			maxSize:   3,
			maxChunks: 10,
			wantCount: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			chunks, err := splitIntoChunks(tt.value, tt.maxSize, tt.maxChunks)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(chunks).To(HaveLen(tt.wantCount))
			// Verify reassembly produces the original value.
			g.Expect(strings.Join(chunks, "")).To(Equal(tt.value))
		})
	}
}

func TestChunkCookieName(t *testing.T) {
	g := NewWithT(t)

	g.Expect(chunkCookieName("auth-storage", 0)).To(Equal("auth-storage"))
	g.Expect(chunkCookieName("auth-storage", 1)).To(Equal("auth-storage-1"))
	g.Expect(chunkCookieName("auth-storage", 9)).To(Equal("auth-storage-9"))
}

func TestAuthStorageChunking(t *testing.T) {
	t.Run("small token uses single cookie", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Insecure: false,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}
		rec := httptest.NewRecorder()
		storage := authStorage{
			AccessToken:  "short-token",
			RefreshToken: "short-refresh",
		}
		g.Expect(setAuthStorage(conf, rec, storage)).To(Succeed())

		cookies := rec.Result().Cookies()
		// Count auth-storage cookies.
		authStorageCookies := 0
		for _, c := range cookies {
			if c.Name == cookieNameAuthStorage || len(c.Name) > len(cookieNameAuthStorage) &&
				c.Name[:len(cookieNameAuthStorage)] == cookieNameAuthStorage {
				authStorageCookies++
			}
		}
		g.Expect(authStorageCookies).To(Equal(1))
	})

	t.Run("large token is chunked and reassembled", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Insecure: false,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}

		// Create a large token that exceeds chunk size.
		largeToken := strings.Repeat("a", cookieChunkMaxSize+500)

		rec := httptest.NewRecorder()
		storage := authStorage{
			AccessToken:  largeToken,
			RefreshToken: "refresh-token",
		}
		g.Expect(setAuthStorage(conf, rec, storage)).To(Succeed())

		cookies := rec.Result().Cookies()

		// Count auth-storage cookies (should be > 1).
		authStorageCookies := 0
		for _, c := range cookies {
			if c.Name == cookieNameAuthStorage || len(c.Name) > len(cookieNameAuthStorage) &&
				c.Name[:len(cookieNameAuthStorage)] == cookieNameAuthStorage {
				authStorageCookies++
			}
		}
		g.Expect(authStorageCookies).To(BeNumerically(">", 1))

		// Create a request with all the cookies.
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, c := range cookies {
			req.AddCookie(c)
		}

		// Verify reassembly.
		result, err := getAuthStorage(req)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.AccessToken).To(Equal(largeToken))
		g.Expect(result.RefreshToken).To(Equal("refresh-token"))
	})

	t.Run("too large token errors out", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Insecure: false,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}

		// Create a large token that exceeds chunk size.
		largeToken := strings.Repeat("12345678901234567890", 10000)

		rec := httptest.NewRecorder()
		storage := authStorage{
			AccessToken:  largeToken,
			RefreshToken: "refresh-token",
		}
		err := setAuthStorage(conf, rec, storage)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("value too large: requires 75 chunks"))
	})

	t.Run("backward compatibility with single cookie", func(t *testing.T) {
		g := NewWithT(t)

		// Simulate old-format single cookie.
		storage := authStorage{
			AccessToken:  "old-token",
			RefreshToken: "old-refresh",
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
		g.Expect(result.AccessToken).To(Equal("old-token"))
		g.Expect(result.RefreshToken).To(Equal("old-refresh"))
	})
}

func TestGetChunkedCookieValue(t *testing.T) {
	t.Run("returns error for missing cookie", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := getChunkedCookieValue(req, "missing-cookie")
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("handles partial chunks gracefully", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test-cookie", Value: "chunk0"})
		req.AddCookie(&http.Cookie{Name: "test-cookie-1", Value: "chunk1"})
		req.AddCookie(&http.Cookie{Name: "test-cookie-2", Value: "chunk2"})
		// Intentionally missing chunk-3 and beyond.

		value, err := getChunkedCookieValue(req, "test-cookie")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(value).To(Equal("chunk0chunk1chunk2"))
	})

	t.Run("reads single cookie when no chunks exist", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "test-cookie", Value: "single-value"})

		value, err := getChunkedCookieValue(req, "test-cookie")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(value).To(Equal("single-value"))
	})
}

func TestClearChunkedCookiesFromResponse(t *testing.T) {
	g := NewWithT(t)

	rec := httptest.NewRecorder()

	// Set multiple chunk cookies.
	http.SetCookie(rec, &http.Cookie{Name: "auth-storage", Value: "chunk0"})
	http.SetCookie(rec, &http.Cookie{Name: "auth-storage-1", Value: "chunk1"})
	http.SetCookie(rec, &http.Cookie{Name: "auth-storage-2", Value: "chunk2"})
	http.SetCookie(rec, &http.Cookie{Name: "other-cookie", Value: "keep"})

	clearChunkedCookiesFromResponse(rec, "auth-storage")

	cookieHeaders := rec.Header().Values("Set-Cookie")
	// Only "other-cookie" should remain.
	g.Expect(cookieHeaders).To(HaveLen(1))
	g.Expect(cookieHeaders[0]).To(ContainSubstring("other-cookie"))
}
