// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

func TestParseIPOrPrefix(t *testing.T) {
	for _, tt := range []struct {
		value    string
		expected netip.Prefix
		wantErr  string
	}{
		{"10.42.1.12", netip.MustParsePrefix("10.42.1.12/32"), ""},
		{"10.42.0.0/16", netip.MustParsePrefix("10.42.0.0/16"), ""},
		{"fd7a:115c:a1e0::1", netip.MustParsePrefix("fd7a:115c:a1e0::1/128"), ""},
		{"not-an-ip", netip.Prefix{}, "expected an IP address or CIDR prefix"},
	} {
		t.Run(tt.value, func(t *testing.T) {
			g := NewWithT(t)
			prefix, err := parseIPOrPrefix(tt.value)
			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(prefix).To(Equal(tt.expected))
		})
	}
}

func TestTrustedProxySet(t *testing.T) {
	g := NewWithT(t)
	set, err := newTrustedProxySet([]string{
		"10.42.123.45/16",
		"192.168.1.10",
		"fd7a:115c:a1e0::/48",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(set.prefixes).To(ConsistOf(
		netip.MustParsePrefix("10.42.0.0/16"),
		netip.MustParsePrefix("192.168.1.10/32"),
		netip.MustParsePrefix("fd7a:115c:a1e0::/48"),
	))
	g.Expect(set.Contains(netip.MustParseAddr("10.42.50.10"))).To(BeTrue())
	g.Expect(set.Contains(netip.MustParseAddr("10.43.0.1"))).To(BeFalse())

	_, err = newTrustedProxySet(nil)
	g.Expect(err).To(MatchError("at least one trusted proxy must be configured"))
	_, err = newTrustedProxySet([]string{"invalid"})
	g.Expect(err).To(MatchError(ContainSubstring(`invalid trusted proxy "invalid"`)))
}

func TestRemoteIP(t *testing.T) {
	for _, tt := range []struct {
		value    string
		expected string
		wantErr  bool
	}{
		{"10.42.1.12:12345", "10.42.1.12", false},
		{"[fd7a:115c:a1e0::1]:12345", "fd7a:115c:a1e0::1", false},
		{"10.42.1.12", "10.42.1.12", false},
		{"invalid", "", true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			g := NewWithT(t)
			addr, err := remoteIP(tt.value)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(addr.String()).To(Equal(tt.expected))
		})
	}
}

func TestNewReverseProxyMiddlewareValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		conf    *fluxcdv1.WebConfigSpec
		wantErr string
	}{
		{"nil config", nil, "reverse proxy authentication is not configured"},
		{"missing trusted proxies", func() *fluxcdv1.WebConfigSpec {
			conf := newTestReverseProxyConfig()
			conf.Authentication.ReverseProxy.TrustedProxies = nil
			return conf
		}(), "failed to configure trusted proxies"},
		{"invalid CEL", func() *fluxcdv1.WebConfigSpec {
			conf := newTestReverseProxyConfig()
			conf.Authentication.ReverseProxy.Impersonation.Username = "invalid[[["
			return conf
		}(), "failed to create claims processor"},
		{"valid config", newTestReverseProxyConfig(), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			middleware, err := newReverseProxyMiddlewareWithClientFactory(
				tt.conf,
				func(user.Impersonation) (any, error) { return &struct{}{}, nil },
			)
			if tt.wantErr != "" {
				g.Expect(middleware).To(BeNil())
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(middleware).NotTo(BeNil())
		})
	}
}

func TestReverseProxyMiddleware(t *testing.T) {
	t.Run("processes canonical header claims with CEL", func(t *testing.T) {
		g := NewWithT(t)
		conf := newTestReverseProxyConfig()
		var received user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				received = imp
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(user.Username(r.Context())).To(Equal("Alice Example"))
			g.Expect(user.Provider(r.Context())).To(Equal(map[string]any{
				"type": fluxcdv1.AuthenticationTypeReverseProxy,
			}))
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header["x-remote-user"] = []string{"alice@example.com"}
		req.Header.Add("X-Remote-Name", "Alice Example")
		req.Header.Set("X-Remote-Groups", "platform,flux-admin")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(received).To(Equal(user.Impersonation{
			Username: "alice@example.com",
			Groups:   []string{"flux-admin", "platform"},
		}))
		g.Expect(rec.Header().Get("Set-Cookie")).To(ContainSubstring("auth-provider="))
	})

	t.Run("uses an earlier variable as the profile name fallback", func(t *testing.T) {
		g := NewWithT(t)
		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			newTestReverseProxyConfig(),
			func(user.Impersonation) (any, error) { return &struct{}{}, nil },
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g.Expect(user.Username(r.Context())).To(Equal("alice@example.com"))
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")
		req.Header.Set("X-Remote-Groups", "platform")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
	})

	for _, tt := range []struct {
		name       string
		remoteAddr string
		username   string
		wantBody   string
	}{
		{
			name:       "rejects invalid peer address",
			remoteAddr: "invalid",
			username:   "alice@example.com",
			wantBody:   "invalid reverse proxy address",
		},
		{
			name:       "rejects untrusted peer",
			remoteAddr: "192.168.1.10:12345",
			username:   "alice@example.com",
			wantBody:   "request did not originate from a trusted reverse proxy",
		},
		{
			name:       "hides validation failure",
			remoteAddr: "10.42.1.12:12345",
			username:   "alice@invalid.example",
			wantBody:   "invalid authenticated identity",
		},
		{
			name:       "hides evaluation failure",
			remoteAddr: "10.42.1.12:12345",
			wantBody:   "invalid authenticated identity",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			clientCalled := false
			middleware, err := newReverseProxyMiddlewareWithClientFactory(
				newTestReverseProxyConfig(),
				func(user.Impersonation) (any, error) {
					clientCalled = true
					return &struct{}{}, nil
				},
			)
			g.Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.username != "" {
				req.Header.Set("X-Remote-User", tt.username)
				req.Header.Set("X-Remote-Name", "Alice Example")
				req.Header.Set("X-Remote-Groups", "platform")
			}
			rec := httptest.NewRecorder()
			middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("next handler must not be called")
			})).ServeHTTP(rec, req)

			g.Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			g.Expect(rec.Body.String()).To(ContainSubstring(tt.wantBody))
			g.Expect(rec.Body.String()).NotTo(ContainSubstring("example.com users only"))
			g.Expect(clientCalled).To(BeFalse())
		})
	}

	t.Run("returns internal error when client creation fails", func(t *testing.T) {
		g := NewWithT(t)
		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			newTestReverseProxyConfig(),
			func(user.Impersonation) (any, error) {
				return nil, errors.New("client failed")
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")
		req.Header.Set("X-Remote-Name", "Alice Example")
		req.Header.Set("X-Remote-Groups", "platform")
		rec := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Fatal("next handler must not be called")
		})).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		g.Expect(rec.Header().Values("Set-Cookie")).To(BeEmpty())
	})
}

func newTestReverseProxyConfig() *fluxcdv1.WebConfigSpec {
	return &fluxcdv1.WebConfigSpec{
		Authentication: &fluxcdv1.AuthenticationSpec{
			Type: fluxcdv1.AuthenticationTypeReverseProxy,
			ReverseProxy: &fluxcdv1.ReverseProxyAuthenticationSpec{
				TrustedProxies: []string{"10.42.0.0/16"},
				ClaimsProcessorSpec: fluxcdv1.ClaimsProcessorSpec{
					Variables: []fluxcdv1.VariableSpec{
						{
							Name:       "username",
							Expression: "claims['X-Remote-User']",
						},
						{
							Name: "groups",
							Expression: "'X-Remote-Groups' in claims " +
								"? claims['X-Remote-Groups'].split(',') : []",
						},
						{
							Name: "name",
							Expression: "'X-Remote-Name' in claims " +
								"? claims['X-Remote-Name'] : variables.username",
						},
					},
					Validations: []fluxcdv1.ValidationSpec{
						{
							Expression: "variables.username.endsWith('@example.com')",
							Message:    "example.com users only",
						},
					},
					Profile: &fluxcdv1.ProfileSpec{
						Name: "variables.name",
					},
					Impersonation: &fluxcdv1.ImpersonationSpec{
						Username: "variables.username",
						Groups:   "variables.groups",
					},
				},
			},
		},
	}
}
