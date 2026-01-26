// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestIsSafeRedirectPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "valid root path",
			path: "/",
			want: true,
		},
		{
			name: "valid simple path",
			path: "/dashboard",
			want: true,
		},
		{
			name: "valid path with query",
			path: "/resource?name=test",
			want: true,
		},
		{
			name: "valid nested path",
			path: "/api/v1/resources",
			want: true,
		},
		{
			name: "protocol-relative URL blocked",
			path: "//evil.com",
			want: false,
		},
		{
			name: "protocol-relative URL with path blocked",
			path: "//evil.com/phishing",
			want: false,
		},
		{
			name: "backslash protocol-relative URL blocked",
			path: "/\\evil.com",
			want: false,
		},
		{
			name: "backslash protocol-relative URL with path blocked",
			path: "/\\evil.com/phishing",
			want: false,
		},
		{
			name: "tab after slash blocked",
			path: "/\tevil.com",
			want: false,
		},
		{
			name: "newline after slash blocked",
			path: "/\nevil.com",
			want: false,
		},
		{
			name: "carriage return after slash blocked",
			path: "/\revil.com",
			want: false,
		},
		{
			name: "triple slash blocked",
			path: "///evil.com",
			want: false,
		},
		{
			name: "null byte after slash blocked",
			path: "/\x00evil.com",
			want: false,
		},
		{
			name: "space after slash blocked",
			path: "/ evil.com",
			want: false,
		},
		{
			name: "absolute URL with http blocked",
			path: "http://evil.com",
			want: false,
		},
		{
			name: "absolute URL with https blocked",
			path: "https://evil.com",
			want: false,
		},
		{
			name: "absolute URL with https and path blocked",
			path: "https://evil.com/phishing",
			want: false,
		},
		{
			name: "javascript scheme blocked",
			path: "javascript://alert(1)",
			want: false,
		},
		{
			name: "data scheme blocked",
			path: "data://text/html,<script>alert(1)</script>",
			want: false,
		},
		{
			name: "relative path without leading slash blocked",
			path: "dashboard",
			want: false,
		},
		{
			name: "empty path blocked",
			path: "",
			want: false,
		},
		{
			name: "path with embedded scheme blocked",
			path: "/redirect?url=https://evil.com",
			want: true, // This is fine, the scheme is in the query not the path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSafeRedirectPath(tt.path); got != tt.want {
				t.Errorf("isSafeRedirectPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestOriginalURL(t *testing.T) {
	tests := []struct {
		name     string
		query    url.Values
		expected string
	}{
		{
			name:     "no original path defaults to root",
			query:    url.Values{},
			expected: "/",
		},
		{
			name:     "valid original path",
			query:    url.Values{authQueryParamOriginalPath: []string{"/dashboard"}},
			expected: "/dashboard",
		},
		{
			name:     "malicious absolute URL blocked",
			query:    url.Values{authQueryParamOriginalPath: []string{"https://evil.com"}},
			expected: "/",
		},
		{
			name:     "malicious protocol-relative URL blocked",
			query:    url.Values{authQueryParamOriginalPath: []string{"//evil.com"}},
			expected: "/",
		},
		{
			name:     "malicious backslash protocol-relative URL blocked",
			query:    url.Values{authQueryParamOriginalPath: []string{"/\\evil.com"}},
			expected: "/",
		},
		{
			name:     "preserves other query params",
			query:    url.Values{authQueryParamOriginalPath: []string{"/dashboard"}, "foo": []string{"bar"}},
			expected: "/dashboard?foo=bar",
		},
		{
			name:     "malicious URL with preserved query params",
			query:    url.Values{authQueryParamOriginalPath: []string{"https://evil.com"}, "foo": []string{"bar"}},
			expected: "/?foo=bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy since originalURL modifies the query
			query := make(url.Values)
			for k, v := range tt.query {
				query[k] = v
			}
			if got := originalURL(query); got != tt.expected {
				t.Errorf("originalURL(%v) = %q, want %q", tt.query, got, tt.expected)
			}
		})
	}
}

// newTestOAuth2Authenticator creates an oauth2Authenticator for testing with a valid GCM cipher.
func newTestOAuth2Authenticator(t *testing.T) *oauth2Authenticator {
	t.Helper()

	secret := []byte("test-client-secret-for-testing-purposes")
	hash := sha256.New
	var salt []byte
	const info = "oauth2 login state cookie encryption"
	key, err := hkdf.Key(hash, secret, salt, info, oauth2LoginStateAESKeySize)
	if err != nil {
		t.Fatalf("failed to derive key: %v", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create GCM: %v", err)
	}

	return &oauth2Authenticator{
		gcm: gcm,
	}
}

func TestOAuth2LoginStateEncoding(t *testing.T) {
	t.Run("round-trip encode/decode preserves state", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestOAuth2Authenticator(t)

		originalState := oauth2LoginState{
			PKCEVerifier: "pkce-verifier-123",
			CSRFToken:    "csrf-token-456",
			Nonce:        "nonce-789",
			URLQuery: url.Values{
				"originalPath": []string{"/dashboard"},
				"param":        []string{"value"},
			},
			ExpiresAt: time.Now().Add(5 * time.Minute).Truncate(time.Second),
		}

		// Encode
		encoded, err := auth.encodeState(originalState)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(encoded).NotTo(BeEmpty())

		// Decode
		decoded, err := auth.decodeState(encoded)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(decoded).NotTo(BeNil())

		// Verify all fields
		g.Expect(decoded.PKCEVerifier).To(Equal(originalState.PKCEVerifier))
		g.Expect(decoded.CSRFToken).To(Equal(originalState.CSRFToken))
		g.Expect(decoded.Nonce).To(Equal(originalState.Nonce))
		g.Expect(decoded.URLQuery).To(Equal(originalState.URLQuery))
		g.Expect(decoded.ExpiresAt.Unix()).To(Equal(originalState.ExpiresAt.Unix()))
	})

	t.Run("decode fails on invalid base64", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestOAuth2Authenticator(t)

		_, err := auth.decodeState("not-valid-base64!!!")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to decode oauth2 login state"))
	})

	t.Run("decode fails on too-short input", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestOAuth2Authenticator(t)

		// Less than 12 bytes (GCM nonce size)
		_, err := auth.decodeState("c2hvcnQ")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("invalid oauth2 login state size"))
	})

	t.Run("decode fails on invalid ciphertext", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestOAuth2Authenticator(t)

		// Valid base64 but not valid encrypted state
		_, err := auth.decodeState("YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("failed to decrypt oauth2 login state"))
	})

	t.Run("different encryptions produce different outputs", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestOAuth2Authenticator(t)

		state := oauth2LoginState{
			PKCEVerifier: "verifier",
			CSRFToken:    "csrf",
			Nonce:        "nonce",
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		}

		encoded1, err := auth.encodeState(state)
		g.Expect(err).NotTo(HaveOccurred())

		encoded2, err := auth.encodeState(state)
		g.Expect(err).NotTo(HaveOccurred())

		// Each encryption should produce different output due to random nonce
		g.Expect(encoded1).NotTo(Equal(encoded2))

		// Both should decode to the same state
		decoded1, _ := auth.decodeState(encoded1)
		decoded2, _ := auth.decodeState(encoded2)
		g.Expect(decoded1.PKCEVerifier).To(Equal(decoded2.PKCEVerifier))
		g.Expect(decoded1.CSRFToken).To(Equal(decoded2.CSRFToken))
		g.Expect(decoded1.Nonce).To(Equal(decoded2.Nonce))
	})
}

func TestOAuth2LoginStateRedirectURL(t *testing.T) {
	for _, tt := range []struct {
		name     string
		state    oauth2LoginState
		expected string
	}{
		{
			name: "returns path from originalPath",
			state: oauth2LoginState{
				URLQuery: url.Values{
					authQueryParamOriginalPath: []string{"/dashboard"},
				},
			},
			expected: "/dashboard",
		},
		{
			name: "falls back to root for missing param",
			state: oauth2LoginState{
				URLQuery: url.Values{},
			},
			expected: "/",
		},
		{
			name: "preserves other query params",
			state: oauth2LoginState{
				URLQuery: url.Values{
					authQueryParamOriginalPath: []string{"/page"},
					"foo":                      []string{"bar"},
				},
			},
			expected: "/page?foo=bar",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := tt.state.redirectURL()
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestVerifyTokenAndSetStorageOrLogErrorOptions(t *testing.T) {
	t.Run("withNonce sets nonce in options", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		withNonce("test-nonce-123")(&opts)

		g.Expect(opts.nonce).To(Equal("test-nonce-123"))
		g.Expect(opts.sessionStart).To(BeNil())
	})

	t.Run("withNonce with empty string sets empty nonce", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		withNonce("")(&opts)

		g.Expect(opts.nonce).To(Equal(""))
		g.Expect(opts.sessionStart).To(BeNil())
	})

	t.Run("withSessionStart sets sessionStart in options", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		sessionTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		withSessionStart(sessionTime)(&opts)

		g.Expect(opts.nonce).To(Equal(""))
		g.Expect(opts.sessionStart).NotTo(BeNil())
		g.Expect(*opts.sessionStart).To(Equal(sessionTime))
	})

	t.Run("multiple options can be applied", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		sessionTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		withNonce("test-nonce")(&opts)
		withSessionStart(sessionTime)(&opts)

		g.Expect(opts.nonce).To(Equal("test-nonce"))
		g.Expect(opts.sessionStart).NotTo(BeNil())
		g.Expect(*opts.sessionStart).To(Equal(sessionTime))
	})

	t.Run("later options override earlier ones", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		sessionTime1 := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		sessionTime2 := time.Date(2025, 6, 20, 14, 45, 0, 0, time.UTC)

		withNonce("first-nonce")(&opts)
		withSessionStart(sessionTime1)(&opts)
		withNonce("second-nonce")(&opts)
		withSessionStart(sessionTime2)(&opts)

		g.Expect(opts.nonce).To(Equal("second-nonce"))
		g.Expect(*opts.sessionStart).To(Equal(sessionTime2))
	})

	t.Run("withSessionStart with zero time sets pointer to zero time", func(t *testing.T) {
		g := NewWithT(t)

		var opts verifyTokenAndSetStorageOrLogErrorOptions
		var zeroTime time.Time
		withSessionStart(zeroTime)(&opts)

		g.Expect(opts.sessionStart).NotTo(BeNil())
		g.Expect(opts.sessionStart.IsZero()).To(BeTrue())
	})
}

// mockOAuth2Verifier is a test double for oauth2Verifier.
type mockOAuth2Verifier struct {
	details *user.Details
	storage *authStorage
	err     error
}

func (m *mockOAuth2Verifier) verifyAccessToken(ctx context.Context, accessToken string, nonce ...string) (*user.Details, error) {
	return m.details, m.err
}

func (m *mockOAuth2Verifier) verifyToken(ctx context.Context, token *oauth2.Token, nonce ...string) (*user.Details, *authStorage, error) {
	return m.details, m.storage, m.err
}

func TestVerifyTokenAndSetStorageOrLogError_SessionStart(t *testing.T) {
	// Helper to create a test oauth2Authenticator
	newTestAuthenticator := func(t *testing.T) *oauth2Authenticator {
		t.Helper()
		auth := newTestOAuth2Authenticator(t)
		auth.conf = &fluxcdv1.WebConfigSpec{
			Insecure: true,
			Authentication: &fluxcdv1.AuthenticationSpec{
				SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			},
		}
		return auth
	}

	t.Run("uses provided non-zero sessionStart", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestAuthenticator(t)
		mockVerifier := &mockOAuth2Verifier{
			details: &user.Details{
				Profile: user.Profile{Name: "Test User"},
			},
			storage: &authStorage{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
			},
		}

		providedTime := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		rec := httptest.NewRecorder()

		details, err := auth.verifyTokenAndSetStorageOrLogError(
			context.Background(), rec, mockVerifier, &oauth2.Token{},
			withSessionStart(providedTime),
		)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(details).NotTo(BeNil())
		g.Expect(details.SessionStart).NotTo(BeNil())
		g.Expect(*details.SessionStart).To(Equal(providedTime))
	})

	t.Run("uses time.Now when sessionStart option is zero", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestAuthenticator(t)
		mockVerifier := &mockOAuth2Verifier{
			details: &user.Details{
				Profile: user.Profile{Name: "Test User"},
			},
			storage: &authStorage{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
			},
		}

		beforeCall := time.Now()
		rec := httptest.NewRecorder()

		var zeroTime time.Time
		details, err := auth.verifyTokenAndSetStorageOrLogError(
			context.Background(), rec, mockVerifier, &oauth2.Token{},
			withSessionStart(zeroTime),
		)
		afterCall := time.Now()

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(details).NotTo(BeNil())
		g.Expect(details.SessionStart).NotTo(BeNil())
		// Session start should be set to approximately time.Now()
		g.Expect(details.SessionStart.After(beforeCall.Add(-time.Second))).To(BeTrue())
		g.Expect(details.SessionStart.Before(afterCall.Add(time.Second))).To(BeTrue())
	})

	t.Run("uses time.Now when no sessionStart option provided", func(t *testing.T) {
		g := NewWithT(t)

		auth := newTestAuthenticator(t)
		mockVerifier := &mockOAuth2Verifier{
			details: &user.Details{
				Profile: user.Profile{Name: "Test User"},
			},
			storage: &authStorage{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
			},
		}

		beforeCall := time.Now()
		rec := httptest.NewRecorder()

		details, err := auth.verifyTokenAndSetStorageOrLogError(
			context.Background(), rec, mockVerifier, &oauth2.Token{},
		)
		afterCall := time.Now()

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(details).NotTo(BeNil())
		g.Expect(details.SessionStart).NotTo(BeNil())
		// Session start should be set to approximately time.Now()
		g.Expect(details.SessionStart.After(beforeCall.Add(-time.Second))).To(BeTrue())
		g.Expect(details.SessionStart.Before(afterCall.Add(time.Second))).To(BeTrue())
	})
}

func TestConsumeOAuth2LoginStates(t *testing.T) {
	t.Run("returns query state and cookie state", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=query-state-123", nil)
		req.AddCookie(&http.Cookie{
			Name:  cookieNameOAuth2LoginState,
			Value: "cookie-state-456",
		})
		rec := httptest.NewRecorder()

		queryState, cookieState := consumeOAuth2LoginStates(rec, req)

		g.Expect(queryState).To(Equal("query-state-123"))
		g.Expect(cookieState).To(Equal("cookie-state-456"))

		// Should delete the cookie
		cookies := rec.Result().Cookies()
		var deletedCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == cookieNameOAuth2LoginState {
				deletedCookie = c
				break
			}
		}
		g.Expect(deletedCookie).NotTo(BeNil())
		g.Expect(deletedCookie.MaxAge).To(Equal(-1))
	})

	t.Run("returns empty string when cookie missing", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?state=query-state-123", nil)
		rec := httptest.NewRecorder()

		queryState, cookieState := consumeOAuth2LoginStates(rec, req)

		g.Expect(queryState).To(Equal("query-state-123"))
		g.Expect(cookieState).To(BeEmpty())
	})

	t.Run("returns empty query state when missing", func(t *testing.T) {
		g := NewWithT(t)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback", nil)
		req.AddCookie(&http.Cookie{
			Name:  cookieNameOAuth2LoginState,
			Value: "cookie-state-456",
		})
		rec := httptest.NewRecorder()

		queryState, cookieState := consumeOAuth2LoginStates(rec, req)

		g.Expect(queryState).To(BeEmpty())
		g.Expect(cookieState).To(Equal("cookie-state-456"))
	})
}
