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

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	oidcProviderRefreshInterval = time.Minute
)

// oidcProvider implements oauth2Provider for OIDC.
type oidcProvider struct {
	conf          *config.ConfigSpec
	processClaims claimsProcessorFunc

	mu        sync.RWMutex
	p         *oidc.Provider
	nextFetch time.Time
}

// newOIDCProvider creates a new OIDC OAuth2 provider.
func newOIDCProvider(conf *config.ConfigSpec) (oauth2Provider, error) {
	processClaims, err := newClaimsProcessor(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create claims processor: %w", err)
	}

	return &oidcProvider{
		conf:          conf,
		processClaims: processClaims,
	}, nil
}

// init implements oauth2Provider.
func (o *oidcProvider) init(ctx context.Context) (initializedOAuth2Provider, error) {
	var p *oidc.Provider

	o.mu.RLock()
	if time.Now().Before(o.nextFetch) {
		p = o.p
	}
	o.mu.RUnlock()

	if p == nil {
		// Fetch without locking to avoid contention.
		var err error
		p, err = oidc.NewProvider(ctx, o.conf.Authentication.OAuth2.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to discover OIDC configuration: %w", err)
		}

		o.mu.Lock()
		o.p = p
		o.nextFetch = time.Now().Add(oidcProviderRefreshInterval)
		o.mu.Unlock()
	}

	return &initializedOIDCProvider{
		conf:          o.conf,
		processClaims: o.processClaims,
		provider:      p,
	}, nil
}

// initializedOIDCProvider implements initializedOAuth2Provider.
type initializedOIDCProvider struct {
	conf          *config.ConfigSpec
	processClaims claimsProcessorFunc
	provider      *oidc.Provider
}

// config implements initializedOAuth2Provider.
func (i *initializedOIDCProvider) config() *oauth2.Config {
	return &oauth2.Config{
		Endpoint: i.provider.Endpoint(),
		Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups"},
	}
}

// newVerifier implements initializedOAuth2Provider.
func (i *initializedOIDCProvider) newVerifier(ctx context.Context) (oauth2Verifier, error) {
	return &oidcVerifier{
		conf:          i.conf,
		verifier:      i.provider.VerifierContext(ctx, &oidc.Config{ClientID: i.conf.Authentication.OAuth2.ClientID}),
		processClaims: i.processClaims,
	}, nil
}

// oidcVerifier implements oauth2Verifier.
type oidcVerifier struct {
	conf          *config.ConfigSpec
	verifier      *oidc.IDTokenVerifier
	processClaims claimsProcessorFunc
}

// verifyAccessToken implements oauth2Verifier.
func (o *oidcVerifier) verifyAccessToken(ctx context.Context,
	accessToken string, nonce ...string) (*user.Details, error) {

	l := log.FromContext(ctx)

	idToken, err := o.verifier.Verify(ctx, accessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify OIDC ID token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims from OIDC ID token: %w", err)
	}
	l.V(1).Info("OIDC authentication successful", "claims", claims)

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
			"claimsProcessor", o.conf.Authentication.OAuth2.ClaimsProcessorSpec)
		return nil, fmt.Errorf("failed to process claims from OIDC ID token: %w", err)
	}
	details.Provider = claims

	return details, nil
}

// verifyToken implements oauth2Verifier.
func (o *oidcVerifier) verifyToken(ctx context.Context,
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
