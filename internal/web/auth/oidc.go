// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	oidcProviderRefreshInterval = time.Minute
)

// oidcProvider implements oauth2Provider for OIDC.
// It refreshes the OIDC provider in a background goroutine
// so that HTTP request handlers never block on OIDC discovery.
type oidcProvider struct {
	conf          *fluxcdv1.WebConfigSpec
	processClaims claimsProcessorFunc
	cancelFunc    context.CancelFunc
	stopped       chan struct{}

	mu          sync.RWMutex
	initialized bool
	lastErr     error
	provider    *oidc.Provider
	cachedVer   *oidc.IDTokenVerifier
}

// newOIDCProvider creates a new OIDC OAuth2 provider and starts
// a background goroutine that periodically refreshes the OIDC
// discovery configuration.
func newOIDCProvider(conf *fluxcdv1.WebConfigSpec) (oauth2Provider, error) {
	processClaims, err := newClaimsProcessor(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create claims processor: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	o := &oidcProvider{
		conf:          conf,
		processClaims: processClaims,
		cancelFunc:    cancel,
		stopped:       make(chan struct{}),
	}
	go o.run(ctx)
	return o, nil
}

// run is the background goroutine that periodically refreshes
// the OIDC provider.
func (o *oidcProvider) run(ctx context.Context) {
	defer close(o.stopped)

	o.refresh(ctx)

	ticker := time.NewTicker(oidcProviderRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.refresh(ctx)
		}
	}
}

// refresh fetches the OIDC discovery configuration and updates
// the cached provider and verifier.
func (o *oidcProvider) refresh(ctx context.Context) {
	p, err := oidc.NewProvider(ctx, o.conf.Authentication.OAuth2.IssuerURL)
	if err != nil {
		// On failure: if already initialized, keep stale data.
		// Only log if the context hasn't been canceled (clean shutdown).
		if ctx.Err() == nil {
			log.FromContext(ctx).Error(err, "failed to refresh OIDC provider")
		}

		o.mu.Lock()
		if !o.initialized {
			o.lastErr = fmt.Errorf("failed to discover OIDC configuration: %w", err)
		}
		o.mu.Unlock()
		return
	}

	ver := p.VerifierContext(ctx, &oidc.Config{
		ClientID: o.conf.Authentication.OAuth2.ClientID,
	})

	o.mu.Lock()
	o.provider = p
	o.cachedVer = ver
	o.initialized = true
	o.lastErr = nil
	o.mu.Unlock()
}

// config implements oauth2Provider.
func (o *oidcProvider) config() (*oauth2.Config, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.initialized {
		if o.lastErr != nil {
			return nil, fmt.Errorf("OIDC provider not yet initialized: %w", o.lastErr)
		}
		return nil, fmt.Errorf("OIDC provider not yet initialized")
	}

	return &oauth2.Config{
		Endpoint: o.provider.Endpoint(),
		Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups"},
	}, nil
}

// close implements oauth2Provider. It stops the background
// goroutine and waits for it to exit or for the context to
// expire, whichever comes first.
func (o *oidcProvider) close(ctx context.Context) error {
	o.cancelFunc()
	select {
	case <-o.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// cachedVerifier returns the cached ID token verifier, or an
// error if the provider has not yet been initialized.
func (o *oidcProvider) cachedVerifier() (*oidc.IDTokenVerifier, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if !o.initialized {
		if o.lastErr != nil {
			return nil, fmt.Errorf("OIDC provider not yet initialized: %w", o.lastErr)
		}
		return nil, fmt.Errorf("OIDC provider not yet initialized")
	}

	return o.cachedVer, nil
}

// verifyAccessToken implements oauth2Verifier.
func (o *oidcProvider) verifyAccessToken(ctx context.Context,
	accessToken string, nonce ...string) (*user.Details, error) {

	v, err := o.cachedVerifier()
	if err != nil {
		return nil, err
	}

	l := log.FromContext(ctx)

	idToken, err := v.Verify(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify OIDC ID token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims from OIDC ID token: %w", err)
	}
	l.V(1).Info("OIDC claims", "claims", claims)

	if len(nonce) > 0 {
		tokenNonce, ok := claims["nonce"]
		if !ok {
			return nil, fmt.Errorf("nonce claim not found in OIDC ID token")
		}
		tokenNonceStr, ok := tokenNonce.(string)
		if !ok {
			return nil, fmt.Errorf("nonce claim in OIDC ID token is not a string")
		}
		if tokenNonceStr != nonce[0] {
			return nil, fmt.Errorf("nonce claim mismatch in OIDC ID token")
		}
	}

	details, err := o.processClaims(ctx, claims)
	if err != nil {
		l.Error(err, "failed to process claims from OIDC ID token",
			"claims", claims,
			"claimsProcessor", o.conf.Authentication.OAuth2.ClaimsProcessorSpec)
		return nil, fmt.Errorf("failed to process claims from OIDC ID token: %w", err)
	}
	details.Provider = claims

	return details, nil
}

// verifyToken implements oauth2Verifier.
func (o *oidcProvider) verifyToken(ctx context.Context,
	token *oauth2.Token, nonce ...string) (*user.Details, *authStorage, error) {

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, nil, fmt.Errorf("no id_token found in token response")
	}

	details, err := o.verifyAccessToken(ctx, idToken, nonce...)
	if err != nil {
		return nil, nil, err
	}

	as := &authStorage{
		AccessToken:  idToken,
		RefreshToken: token.RefreshToken,
	}

	return details, as, nil
}
