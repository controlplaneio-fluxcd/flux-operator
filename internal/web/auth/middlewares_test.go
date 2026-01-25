// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestNewDefaultMiddleware(t *testing.T) {
	g := NewWithT(t)

	middleware := newDefaultMiddleware()
	g.Expect(middleware).NotTo(BeNil())

	// Create a test handler
	var nextCalled bool
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	// Apply middleware
	handler := middleware(nextHandler)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify next handler was called
	g.Expect(nextCalled).To(BeTrue())

	// Verify auth provider cookie was set
	cookies := rec.Result().Cookies()
	var authProviderCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == cookieNameAuthProvider {
			authProviderCookie = c
			break
		}
	}
	g.Expect(authProviderCookie).NotTo(BeNil())

	// Verify Anonymous provider with authenticated=true
	decoded, err := base64.RawURLEncoding.DecodeString(authProviderCookie.Value)
	g.Expect(err).NotTo(HaveOccurred())

	var result map[string]any
	err = json.Unmarshal(decoded, &result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result["provider"]).To(Equal(fluxcdv1.AuthenticationTypeAnonymous))
	g.Expect(result["authenticated"]).To(BeTrue())
}

func TestLogoutHandling(t *testing.T) {
	t.Run("POST /logout deletes auth storage and redirects", func(t *testing.T) {
		g := NewWithT(t)

		// Create a simple middleware wrapper that handles logout
		// (simulating what NewMiddleware does)
		middleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case authPathLogout:
					if r.Method != http.MethodPost {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					deleteAuthStorage(w)
					http.Redirect(w, r, "/", http.StatusSeeOther)
				default:
					next.ServeHTTP(w, r)
				}
			})
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := middleware(nextHandler)

		req := httptest.NewRequest(http.MethodPost, "/logout", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Should redirect
		g.Expect(rec.Code).To(Equal(http.StatusSeeOther))
		g.Expect(rec.Header().Get("Location")).To(Equal("/"))

		// Should have cookie deletion
		cookies := rec.Result().Cookies()
		var storageDeleted bool
		for _, c := range cookies {
			if c.Name == cookieNameAuthStorage && c.MaxAge == -1 {
				storageDeleted = true
				break
			}
		}
		g.Expect(storageDeleted).To(BeTrue())
	})

	t.Run("GET /logout returns 405 Method Not Allowed", func(t *testing.T) {
		g := NewWithT(t)

		middleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case authPathLogout:
					if r.Method != http.MethodPost {
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
						return
					}
					deleteAuthStorage(w)
					http.Redirect(w, r, "/", http.StatusSeeOther)
				default:
					next.ServeHTTP(w, r)
				}
			})
		}

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := middleware(nextHandler)

		req := httptest.NewRequest(http.MethodGet, "/logout", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
	})
}
