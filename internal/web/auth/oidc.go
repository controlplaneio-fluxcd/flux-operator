// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

// oidcProvider implements oauth2Provider for OIDC.
type oidcProvider struct {
	conf          *config.ConfigSpec
	processClaims claimsProcessorFunc
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
	p, err := oidc.NewProvider(ctx, o.conf.Authentication.OAuth2.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC configuration: %w", err)
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

// verifyAccessToken implements initializedOAuth2Provider.
func (i *initializedOIDCProvider) verifyAccessToken(ctx context.Context, accessToken string) (string, []string, error) {
	v := i.provider.VerifierContext(ctx, &oidc.Config{ClientID: i.conf.Authentication.OAuth2.ClientID})
	idToken, err := v.Verify(ctx, accessToken)
	if err != nil {
		return "", nil, fmt.Errorf("failed to verify OIDC ID token: %w", err)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return "", nil, fmt.Errorf("failed to extract claims from OIDC ID token: %w", err)
	}

	cr, err := i.processClaims(ctx, claims)
	if err != nil {
		return "", nil, fmt.Errorf("failed to process claims from OIDC ID token: %w", err)
	}

	return cr.username, cr.groups, nil
}

// verifyToken implements initializedOAuth2Provider.
func (i *initializedOIDCProvider) verifyToken(ctx context.Context, token *oauth2.Token) (string, []string, *authStorage, error) {
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", nil, nil, fmt.Errorf("no id_token found in token response")
	}
	username, groups, err := i.verifyAccessToken(ctx, idToken)
	if err != nil {
		return "", nil, nil, err
	}
	as := &authStorage{
		// If in the future we implement profile pictures, we will need to store the real
		// access token as well (and not replace it with the ID token, like we do here),
		// because the OIDC /userinfo endpoint only accepts the real access token in some
		// OIDC providers (e.g. Dex). Some OIDC providers return the 'picture' claim in the
		// ID token (e.g. Dex https://github.com/dexidp/dex/issues/4447), so we need to try
		// it first and only fall back to the /userinfo endpoint if it's not present.
		AccessToken:  idToken,
		RefreshToken: token.RefreshToken,
	}
	return username, groups, as, nil
}

// config implements initializedOAuth2Provider.
func (i *initializedOIDCProvider) config() *oauth2.Config {
	return &oauth2.Config{
		Endpoint: i.provider.Endpoint(),
		Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups"},
	}
}
