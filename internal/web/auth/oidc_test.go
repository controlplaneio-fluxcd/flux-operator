// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

// newTestOIDCDiscoveryServer creates an httptest.Server that serves
// a minimal OIDC discovery document. The issuer URL is set to the
// server's own URL.
func newTestOIDCDiscoveryServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var serverURL atomic.Value

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		url := serverURL.Load().(string)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 url,
			"authorization_endpoint": url + "/authorize",
			"token_endpoint":         url + "/token",
			"jwks_uri":               url + "/keys",
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	s := httptest.NewServer(mux)
	serverURL.Store(s.URL)
	t.Cleanup(s.Close)
	return s
}

// newTestWebConfigSpec creates a minimal WebConfigSpec pointing at the
// given issuer URL for OIDC discovery.
func newTestWebConfigSpec(issuerURL string) *fluxcdv1.WebConfigSpec {
	oauth2 := &fluxcdv1.OAuth2AuthenticationSpec{
		Provider:  fluxcdv1.OAuth2ProviderOIDC,
		ClientID:  "test-client-id",
		IssuerURL: issuerURL,
	}
	config.ApplyOAuth2AuthenticationSpecDefaults(oauth2)
	return &fluxcdv1.WebConfigSpec{
		Authentication: &fluxcdv1.AuthenticationSpec{
			OAuth2: oauth2,
		},
	}
}

func TestOIDCProvider_SuccessfulInit(t *testing.T) {
	g := NewWithT(t)

	s := newTestOIDCDiscoveryServer(t)
	conf := newTestWebConfigSpec(s.URL)

	provider, err := newOIDCProvider(conf)
	g.Expect(err).NotTo(HaveOccurred())
	defer provider.close(t.Context())

	// Wait for the background goroutine to complete the first fetch.
	g.Eventually(func() error {
		_, err := provider.config()
		return err
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())

	// config() should return a valid config.
	cfg, err := provider.config()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg).NotTo(BeNil())
	g.Expect(cfg.Endpoint.AuthURL).To(Equal(s.URL + "/authorize"))
	g.Expect(cfg.Endpoint.TokenURL).To(Equal(s.URL + "/token"))

	// cachedVerifier() should return a valid verifier.
	ver, err := provider.(*oidcProvider).cachedVerifier()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ver).NotTo(BeNil())
}

func TestOIDCProvider_PreInitErrors(t *testing.T) {
	g := NewWithT(t)

	// Use a server that will be slow to respond — we'll call
	// config()/verifier() before the first fetch can complete.
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer slowServer.Close()

	conf := newTestWebConfigSpec(slowServer.URL)

	provider, err := newOIDCProvider(conf)
	g.Expect(err).NotTo(HaveOccurred())
	defer provider.close(t.Context())

	// Immediately calling config()/verifyAccessToken() should fail
	// because the provider hasn't initialized yet.
	_, err = provider.config()
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not yet initialized"))

	_, err = provider.verifyAccessToken(context.Background(), "dummy-token")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not yet initialized"))
}

func TestOIDCProvider_FirstFetchFailure(t *testing.T) {
	g := NewWithT(t)

	// Start a server that always returns errors.
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	conf := newTestWebConfigSpec(errorServer.URL)

	provider, err := newOIDCProvider(conf)
	g.Expect(err).NotTo(HaveOccurred())
	defer provider.close(t.Context())

	// Wait a bit for the first fetch attempt to complete and fail.
	g.Eventually(func() string {
		_, err := provider.config()
		if err != nil {
			return err.Error()
		}
		return ""
	}, 5*time.Second, 50*time.Millisecond).Should(ContainSubstring("failed to discover OIDC configuration"))

	_, err = provider.verifyAccessToken(context.Background(), "dummy-token")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to discover OIDC configuration"))
}

func TestOIDCProvider_StaleDataPreservedOnRefreshFailure(t *testing.T) {
	g := NewWithT(t)

	// Start a server that initially works, then fails.
	var shouldFail atomic.Bool
	mux := http.NewServeMux()
	var serverURL atomic.Value

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		if shouldFail.Load() {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		url := serverURL.Load().(string)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 url,
			"authorization_endpoint": url + "/authorize",
			"token_endpoint":         url + "/token",
			"jwks_uri":               url + "/keys",
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	s := httptest.NewServer(mux)
	serverURL.Store(s.URL)
	defer s.Close()

	// Use a very short refresh interval for this test.
	origInterval := oidcProviderRefreshInterval
	defer func() {
		// We can't actually change the const, so this test relies on
		// the provider being created with the module-level const.
		// Instead, we test by closing the provider and creating a new
		// one after the server starts failing.
		_ = origInterval
	}()

	conf := newTestWebConfigSpec(s.URL)

	provider, err := newOIDCProvider(conf)
	g.Expect(err).NotTo(HaveOccurred())
	defer provider.close(t.Context())

	// Wait for successful init.
	g.Eventually(func() error {
		_, err := provider.config()
		return err
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())

	// Record the working config.
	goodCfg, err := provider.config()
	g.Expect(err).NotTo(HaveOccurred())

	// Now break the server.
	shouldFail.Store(true)

	// Force a refresh by closing the old provider and creating a
	// new one — but actually, the better approach is to verify the
	// existing provider's data stays valid. The background goroutine
	// will try to refresh on its ticker interval (1 minute), which
	// is too long for a test. Instead, we directly call refresh
	// to simulate a failed refresh attempt.
	p := provider.(*oidcProvider)
	p.refresh(t.Context())

	// The config should still work with the old good data.
	cfg, err := provider.config()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cfg.Endpoint.AuthURL).To(Equal(goodCfg.Endpoint.AuthURL))
	g.Expect(cfg.Endpoint.TokenURL).To(Equal(goodCfg.Endpoint.TokenURL))

	// Cached verifier should also still work.
	ver, err := provider.(*oidcProvider).cachedVerifier()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ver).NotTo(BeNil())
}

func TestOIDCProvider_CloseStopsGoroutine(t *testing.T) {
	g := NewWithT(t)

	s := newTestOIDCDiscoveryServer(t)
	conf := newTestWebConfigSpec(s.URL)

	provider, err := newOIDCProvider(conf)
	g.Expect(err).NotTo(HaveOccurred())

	// Wait for init.
	g.Eventually(func() error {
		_, err := provider.config()
		return err
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())

	// Close the provider and verify the stopped channel is closed.
	p := provider.(*oidcProvider)
	g.Expect(provider.close(t.Context())).To(Succeed())

	// The stopped channel should be closed (i.e., receive immediately).
	select {
	case <-p.stopped:
		// Success — goroutine has stopped.
	default:
		t.Fatal("stopped channel was not closed after close()")
	}
}
