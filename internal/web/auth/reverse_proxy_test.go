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
	tests := []struct {
		name     string
		value    string
		expected netip.Prefix
		wantErr  string
	}{
		{
			name:     "IPv4 address",
			value:    "10.42.1.12",
			expected: netip.MustParsePrefix("10.42.1.12/32"),
		},
		{
			name:     "IPv4 prefix",
			value:    "10.42.0.0/16",
			expected: netip.MustParsePrefix("10.42.0.0/16"),
		},
		{
			name:     "IPv6 address",
			value:    "fd7a:115c:a1e0::1",
			expected: netip.MustParsePrefix("fd7a:115c:a1e0::1/128"),
		},
		{
			name:     "IPv6 prefix",
			value:    "fd7a:115c:a1e0::/48",
			expected: netip.MustParsePrefix("fd7a:115c:a1e0::/48"),
		},
		{
			name:    "invalid address",
			value:   "not-an-ip",
			wantErr: "expected an IP address or CIDR prefix",
		},
		{
			name:    "invalid prefix",
			value:   "10.42.0.0/99",
			wantErr: "expected an IP address or CIDR prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func TestNewTrustedProxySet(t *testing.T) {
	t.Run("accepts addresses and prefixes", func(t *testing.T) {
		g := NewWithT(t)

		set, err := newTrustedProxySet([]string{
			" 10.42.1.12 ",
			"10.43.0.0/16",
			"fd7a:115c:a1e0::/48",
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(set).NotTo(BeNil())
		g.Expect(set.prefixes).To(ConsistOf(
			netip.MustParsePrefix("10.42.1.12/32"),
			netip.MustParsePrefix("10.43.0.0/16"),
			netip.MustParsePrefix("fd7a:115c:a1e0::/48"),
		))
	})

	t.Run("masks prefixes", func(t *testing.T) {
		g := NewWithT(t)

		set, err := newTrustedProxySet([]string{
			"10.42.123.45/16",
		})

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(set.prefixes).To(Equal([]netip.Prefix{
			netip.MustParsePrefix("10.42.0.0/16"),
		}))
	})

	t.Run("rejects empty configuration", func(t *testing.T) {
		g := NewWithT(t)

		set, err := newTrustedProxySet(nil)

		g.Expect(set).To(BeNil())
		g.Expect(err).To(MatchError(
			"at least one trusted proxy must be configured",
		))
	})

	t.Run("rejects empty entry", func(t *testing.T) {
		g := NewWithT(t)

		set, err := newTrustedProxySet([]string{
			"10.42.0.0/16",
			" ",
		})

		g.Expect(set).To(BeNil())
		g.Expect(err).To(MatchError(
			"trusted proxy at index 1 is empty",
		))
	})

	t.Run("rejects invalid entry", func(t *testing.T) {
		g := NewWithT(t)

		set, err := newTrustedProxySet([]string{
			"invalid",
		})

		g.Expect(set).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring(
			`invalid trusted proxy "invalid"`,
		)))
	})
}

func TestTrustedProxySetContains(t *testing.T) {
	set, err := newTrustedProxySet([]string{
		"10.42.0.0/16",
		"192.168.1.10",
		"fd7a:115c:a1e0::/48",
	})
	if err != nil {
		t.Fatalf("failed to create trusted proxy set: %v", err)
	}

	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "IPv4 address inside prefix",
			address:  "10.42.50.10",
			expected: true,
		},
		{
			name:     "IPv4 network boundary",
			address:  "10.42.0.0",
			expected: true,
		},
		{
			name:     "IPv4 final address in prefix",
			address:  "10.42.255.255",
			expected: true,
		},
		{
			name:     "IPv4 address outside prefix",
			address:  "10.43.0.1",
			expected: false,
		},
		{
			name:     "exact IPv4 address",
			address:  "192.168.1.10",
			expected: true,
		},
		{
			name:     "different IPv4 address",
			address:  "192.168.1.11",
			expected: false,
		},
		{
			name:     "IPv6 address inside prefix",
			address:  "fd7a:115c:a1e0::1234",
			expected: true,
		},
		{
			name:     "IPv6 address outside prefix",
			address:  "fd7a:115c:a1e1::1",
			expected: false,
		},
		{
			name:     "IPv4-mapped IPv6 address",
			address:  "::ffff:10.42.1.2",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := set.Contains(netip.MustParseAddr(tt.address))

			g.Expect(got).To(Equal(tt.expected))
		})
	}

	t.Run("nil set rejects address", func(t *testing.T) {
		g := NewWithT(t)

		var nilSet *trustedProxySet

		g.Expect(nilSet.Contains(
			netip.MustParseAddr("10.42.1.2"),
		)).To(BeFalse())
	})

	t.Run("invalid address is rejected", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(set.Contains(netip.Addr{})).To(BeFalse())
	})
}

func TestRemoteIP(t *testing.T) {
	tests := []struct {
		name     string
		remote   string
		expected string
		wantErr  string
	}{
		{
			name:     "IPv4 with port",
			remote:   "10.42.1.12:54321",
			expected: "10.42.1.12",
		},
		{
			name:     "bare IPv4",
			remote:   "10.42.1.12",
			expected: "10.42.1.12",
		},
		{
			name:     "IPv6 with port",
			remote:   "[fd7a:115c:a1e0::1]:54321",
			expected: "fd7a:115c:a1e0::1",
		},
		{
			name:     "bare IPv6",
			remote:   "fd7a:115c:a1e0::1",
			expected: "fd7a:115c:a1e0::1",
		},
		{
			name:     "bracketed bare IPv6",
			remote:   "[fd7a:115c:a1e0::1]",
			expected: "fd7a:115c:a1e0::1",
		},
		{
			name:     "IPv4-mapped IPv6",
			remote:   "[::ffff:10.42.1.12]:54321",
			expected: "10.42.1.12",
		},
		{
			name:    "empty address",
			remote:  "",
			wantErr: "remote address is empty",
		},
		{
			name:    "invalid host with port",
			remote:  "not-an-ip:54321",
			wantErr: "failed to parse remote IP",
		},
		{
			name:    "invalid bare address",
			remote:  "not-an-ip",
			wantErr: "failed to parse remote address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			addr, err := remoteIP(tt.remote)

			if tt.wantErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.wantErr)))
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(addr.String()).To(Equal(tt.expected))
		})
	}
}

func TestParseHeaderList(t *testing.T) {
	tests := []struct {
		name         string
		headerValues []string
		config       *fluxcdv1.HeaderListSpec
		expected     []string
	}{
		{
			name:         "nil header values",
			headerValues: nil,
			expected:     nil,
		},
		{
			name:         "empty header value",
			headerValues: []string{""},
			expected:     nil,
		},
		{
			name:         "default comma separator",
			headerValues: []string{"platform,developers,flux-admin"},
			expected:     []string{"developers", "flux-admin", "platform"},
		},
		{
			name: "multiple header values",
			headerValues: []string{
				"platform, developers",
				"flux-admin",
			},
			expected: []string{"developers", "flux-admin", "platform"},
		},
		{
			name: "trims and removes empty values",
			headerValues: []string{
				" platform, , developers ,, ",
			},
			expected: []string{"developers", "platform"},
		},
		{
			name: "deduplicates groups",
			headerValues: []string{
				"platform,developers",
				"platform",
				"developers,flux-admin",
			},
			expected: []string{"developers", "flux-admin", "platform"},
		},
		{
			name:         "custom separator",
			headerValues: []string{"platform|developers|flux-admin"},
			config: &fluxcdv1.HeaderListSpec{
				Separator: "|",
			},
			expected: []string{"developers", "flux-admin", "platform"},
		},
		{
			name:         "multi-character separator",
			headerValues: []string{"platform::developers::flux-admin"},
			config: &fluxcdv1.HeaderListSpec{
				Separator: "::",
			},
			expected: []string{"developers", "flux-admin", "platform"},
		},
		{
			name:         "sorts values",
			headerValues: []string{"zeta,alpha,beta"},
			expected:     []string{"alpha", "beta", "zeta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := parseHeaderList(tt.headerValues, tt.config)

			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestNewReverseProxyMiddlewareValidation(t *testing.T) {
	t.Run("rejects nil configuration", func(t *testing.T) {
		g := NewWithT(t)

		middleware, err := newReverseProxyMiddleware(nil, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(
			"reverse proxy authentication is not configured",
		))
	})

	t.Run("rejects nil authentication", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{}

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(
			"reverse proxy authentication is not configured",
		))
	})

	t.Run("rejects nil reverse proxy configuration", func(t *testing.T) {
		g := NewWithT(t)

		conf := &fluxcdv1.WebConfigSpec{
			Authentication: &fluxcdv1.AuthenticationSpec{},
		}

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(
			"reverse proxy authentication is not configured",
		))
	})

	t.Run("rejects missing username header", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.Headers.Username = ""

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(
			"reverse proxy username header must be configured",
		))
	})

	t.Run("rejects whitespace username header", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.Headers.Username = " "

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(
			"reverse proxy username header must be configured",
		))
	})

	t.Run("rejects missing trusted proxies", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.TrustedProxies = nil

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring(
			"failed to configure trusted proxies",
		)))
	})

	t.Run("rejects invalid trusted proxy", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.TrustedProxies = []string{
			"invalid",
		}

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(middleware).To(BeNil())
		g.Expect(err).To(MatchError(ContainSubstring(
			`invalid trusted proxy "invalid"`,
		)))
	})

	t.Run("creates middleware for valid configuration", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()

		middleware, err := newReverseProxyMiddleware(conf, nil)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(middleware).NotTo(BeNil())
	})
}

func TestReverseProxyMiddlewareRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		headers        map[string][]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "invalid remote address",
			remoteAddr:     "invalid-address",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid reverse proxy address",
		},
		{
			name:           "untrusted IPv4 peer",
			remoteAddr:     "192.168.1.10:12345",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "request did not originate from a trusted reverse proxy",
		},
		{
			name:       "spoofed forwarded-for does not bypass peer validation",
			remoteAddr: "192.168.1.10:12345",
			headers: map[string][]string{
				"X-Forwarded-For": {"10.42.1.12"},
				"X-Remote-User":   {"alice@example.com"},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "request did not originate from a trusted reverse proxy",
		},
		{
			name:           "missing username header",
			remoteAddr:     "10.42.1.12:12345",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "authenticated username header is missing",
		},
		{
			name:       "empty username header",
			remoteAddr: "10.42.1.12:12345",
			headers: map[string][]string{
				"X-Remote-User": {" "},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "authenticated username header is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conf := newTestReverseProxyConfig()

			middleware, err := newReverseProxyMiddlewareWithClientFactory(
				conf,
				func(user.Impersonation) (any, error) {
					return &struct{}{}, nil
				},
			)
			g.Expect(err).NotTo(HaveOccurred())

			nextCalled := false
			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for name, values := range tt.headers {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}

			rec := httptest.NewRecorder()
			middleware(next).ServeHTTP(rec, req)

			g.Expect(rec.Code).To(Equal(tt.expectedStatus))
			g.Expect(rec.Body.String()).To(ContainSubstring(tt.expectedBody))
			g.Expect(nextCalled).To(BeFalse())
		})
	}
}

func TestReverseProxyMiddlewareAuthenticatedRequest(t *testing.T) {
	t.Run("uses supplied profile name and parsed groups", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()

		expectedClient := &struct {
			name string
		}{
			name: "test-client",
		}

		var receivedImpersonation user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				receivedImpersonation = imp
				return expectedClient, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		nextCalled := false
		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			nextCalled = true

			session := user.LoadSession(r.Context())
			g.Expect(session).NotTo(BeNil())
			g.Expect(session.Name).To(Equal("Alice Example"))

			g.Expect(user.Username(r.Context())).To(Equal("Alice Example"))
			g.Expect(user.Permissions(r.Context())).To(Equal(
				user.Impersonation{
					Username: "alice@example.com",
					Groups: []string{
						"developers",
						"flux-admin",
						"platform",
					},
				},
			))
			g.Expect(user.Provider(r.Context())).To(Equal(
				map[string]any{
					"type": fluxcdv1.AuthenticationTypeReverseProxy,
				},
			))
			g.Expect(user.KubeClient(r.Context())).To(BeIdenticalTo(
				expectedClient,
			))

			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", " alice@example.com ")
		req.Header.Set("X-Remote-Name", " Alice Example ")
		req.Header.Add("X-Remote-Groups", "platform, developers")
		req.Header.Add("X-Remote-Groups", "flux-admin,platform")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(nextCalled).To(BeTrue())
		g.Expect(receivedImpersonation).To(Equal(
			user.Impersonation{
				Username: "alice@example.com",
				Groups: []string{
					"developers",
					"flux-admin",
					"platform",
				},
			},
		))

		g.Expect(rec.Header().Values("Set-Cookie")).NotTo(BeEmpty())
		g.Expect(rec.Header().Get("Set-Cookie")).To(
			ContainSubstring("auth-provider="),
		)
	})

	t.Run("falls back to username when name header is absent", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(user.Impersonation) (any, error) {
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			session := user.LoadSession(r.Context())
			g.Expect(session).NotTo(BeNil())
			g.Expect(session.Name).To(Equal("alice@example.com"))
			g.Expect(user.Username(r.Context())).To(Equal(
				"alice@example.com",
			))

			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
	})

	t.Run("falls back to username when name header is empty", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(user.Impersonation) (any, error) {
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			session := user.LoadSession(r.Context())
			g.Expect(session).NotTo(BeNil())
			g.Expect(session.Name).To(Equal("alice@example.com"))

			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")
		req.Header.Set("X-Remote-Name", "   ")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
	})

	t.Run("works without groups or default groups", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.DefaultGroups = nil

		var receivedImpersonation user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				receivedImpersonation = imp
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			g.Expect(user.Permissions(r.Context())).To(Equal(
				user.Impersonation{
					Username: "alice@example.com",
					Groups:   nil,
				},
			))

			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(receivedImpersonation).To(Equal(
			user.Impersonation{
				Username: "alice@example.com",
				Groups:   nil,
			},
		))
	})

	t.Run("uses default groups when groups header is absent", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.DefaultGroups = []string{
			"flux-users",
			"developers",
		}

		var receivedImpersonation user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				receivedImpersonation = imp
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			g.Expect(user.Permissions(r.Context())).To(Equal(
				user.Impersonation{
					Username: "alice@example.com",
					Groups: []string{
						"developers",
						"flux-users",
					},
				},
			))

			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(receivedImpersonation.Groups).To(Equal([]string{
			"developers",
			"flux-users",
		}))
	})

	t.Run("uses default groups when groups header contains empty values", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.DefaultGroups = []string{
			"flux-users",
		}

		var receivedImpersonation user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				receivedImpersonation = imp
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")
		req.Header.Set("X-Remote-Groups", " , , ")

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(receivedImpersonation.Groups).To(Equal([]string{
			"flux-users",
		}))
	})

	t.Run("proxy groups override default groups", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.DefaultGroups = []string{
			"flux-users",
		}

		var receivedImpersonation user.Impersonation

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(imp user.Impersonation) (any, error) {
				receivedImpersonation = imp
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		next := http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")
		req.Header.Set(
			"X-Remote-Groups",
			"platform,flux-admin",
		)

		rec := httptest.NewRecorder()
		middleware(next).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(receivedImpersonation.Groups).To(Equal([]string{
			"flux-admin",
			"platform",
		}))
	})

	t.Run("does not mutate configured default groups", func(t *testing.T) {
		g := NewWithT(t)

		conf := newTestReverseProxyConfig()
		conf.Authentication.ReverseProxy.DefaultGroups = []string{
			"flux-users",
			"developers",
		}

		originalDefaultGroups := append(
			[]string(nil),
			conf.Authentication.ReverseProxy.DefaultGroups...,
		)

		middleware, err := newReverseProxyMiddlewareWithClientFactory(
			conf,
			func(user.Impersonation) (any, error) {
				return &struct{}{}, nil
			},
		)
		g.Expect(err).NotTo(HaveOccurred())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.42.1.12:12345"
		req.Header.Set("X-Remote-User", "alice@example.com")

		rec := httptest.NewRecorder()
		middleware(http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusNoContent)
		})).ServeHTTP(rec, req)

		g.Expect(rec.Code).To(Equal(http.StatusNoContent))
		g.Expect(
			conf.Authentication.ReverseProxy.DefaultGroups,
		).To(Equal(originalDefaultGroups))
	})
}

func TestReverseProxyMiddlewareClientFailure(t *testing.T) {
	g := NewWithT(t)

	conf := newTestReverseProxyConfig()

	expectedErr := errors.New("client creation failed")
	var receivedImpersonation user.Impersonation

	middleware, err := newReverseProxyMiddlewareWithClientFactory(
		conf,
		func(imp user.Impersonation) (any, error) {
			receivedImpersonation = imp
			return nil, expectedErr
		},
	)
	g.Expect(err).NotTo(HaveOccurred())

	nextCalled := false
	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.42.1.12:12345"
	req.Header.Set("X-Remote-User", "alice@example.com")
	req.Header.Set("X-Remote-Name", "Alice Example")
	req.Header.Set(
		"X-Remote-Groups",
		"platform,flux-admin",
	)

	rec := httptest.NewRecorder()
	middleware(next).ServeHTTP(rec, req)

	g.Expect(rec.Code).To(Equal(http.StatusInternalServerError))
	g.Expect(rec.Body.String()).To(ContainSubstring(
		"failed to initialize Kubernetes client",
	))
	g.Expect(nextCalled).To(BeFalse())
	g.Expect(receivedImpersonation).To(Equal(
		user.Impersonation{
			Username: "alice@example.com",
			Groups: []string{
				"flux-admin",
				"platform",
			},
		},
	))

	// The provider cookie is only set after the impersonated Kubernetes
	// client has been initialized successfully.
	g.Expect(rec.Header().Values("Set-Cookie")).To(BeEmpty())
}

func newTestReverseProxyConfig() *fluxcdv1.WebConfigSpec {
	return &fluxcdv1.WebConfigSpec{
		Authentication: &fluxcdv1.AuthenticationSpec{
			Type: fluxcdv1.AuthenticationTypeReverseProxy,
			ReverseProxy: &fluxcdv1.ReverseProxyAuthenticationSpec{
				Headers: fluxcdv1.ReverseProxyHeadersSpec{
					Username: "X-Remote-User",
					Name:     "X-Remote-Name",
					Groups:   "X-Remote-Groups",
				},
				Groups: &fluxcdv1.HeaderListSpec{
					Separator: ",",
				},
				TrustedProxies: []string{
					"10.42.0.0/16",
					"fd7a:115c:a1e0::/48",
				},
			},
		},
	}
}
