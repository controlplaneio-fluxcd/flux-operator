// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package web

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/fluxcd/pkg/runtime/cel"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

// newOIDCProvider creates a new OIDC OAuth2 provider.
func newOIDCProvider(conf *config.ConfigSpec) (*oauth2Provider, error) {
	// We get the provider on each request to ensure we have the latest keys.
	// Caching could be implemented here if needed.
	getProvider := func(ctx context.Context) (*oidc.Provider, error) {
		p, err := oidc.NewProvider(ctx, conf.Authentication.OAuth2.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}
		return p, nil
	}

	// Build CEL expressions.
	type variable struct {
		name string
		expr *cel.Expression
	}
	type validation struct {
		expr *cel.Expression
		msg  string
	}
	type impersonation struct {
		username *cel.Expression
		groups   *cel.Expression
	}
	variableExprs := make([]variable, 0, len(conf.Authentication.OAuth2.Variables))
	for _, v := range conf.Authentication.OAuth2.Variables {
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, err
		}
		variableExprs = append(variableExprs, variable{name: v.Name, expr: expr})
	}
	validationExprs := make([]validation, 0, len(conf.Authentication.OAuth2.Validations))
	for _, v := range conf.Authentication.OAuth2.Validations {
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, err
		}
		validationExprs = append(validationExprs, validation{expr: expr, msg: v.Message})
	}
	var impersonationExprs impersonation
	if s := conf.Authentication.OAuth2.Impersonation.Username; s != "" {
		expr, err := cel.NewExpression(s)
		if err != nil {
			return nil, err
		}
		impersonationExprs.username = expr
	}
	if s := conf.Authentication.OAuth2.Impersonation.Groups; s != "" {
		expr, err := cel.NewExpression(s)
		if err != nil {
			return nil, err
		}
		impersonationExprs.groups = expr
	}

	// Build verification function using the CEL expressions.
	verifyAccessToken := func(ctx context.Context, accessToken string) (string, []string, error) {
		p, err := getProvider(ctx)
		if err != nil {
			return "", nil, err
		}

		v := p.VerifierContext(ctx, &oidc.Config{ClientID: conf.Authentication.OAuth2.ClientID})
		idToken, err := v.Verify(ctx, accessToken)
		if err != nil {
			return "", nil, fmt.Errorf("failed to verify OIDC ID token: %w", err)
		}

		var claims map[string]any
		if err := idToken.Claims(&claims); err != nil {
			return "", nil, fmt.Errorf("failed to extract claims from OIDC ID token: %w", err)
		}

		// Extract variables from claims using CEL expressions.
		variables := map[string]any{}
		data := map[string]any{
			"claims":    claims,
			"variables": variables,
		}
		for _, x := range variableExprs {
			value, err := x.expr.Evaluate(ctx, data)
			if err != nil {
				return "", nil, fmt.Errorf("failed to evaluate variable '%s': %w", x.name, err)
			}
			variables[x.name] = value
		}

		// Validate claims and variables using CEL expressions.
		for _, v := range validationExprs {
			result, err := v.expr.EvaluateBoolean(ctx, data)
			if err != nil {
				return "", nil, fmt.Errorf("failed to evaluate validation expression: %w", err)
			}
			if !result {
				return "", nil, fmt.Errorf("validation failed: %s", v.msg)
			}
		}

		// Extract impersonation info using CEL expressions.
		var username string
		var groups []string
		if impersonationExprs.username != nil {
			username, err = impersonationExprs.username.EvaluateString(ctx, data)
			if err != nil {
				return "", nil, fmt.Errorf("failed to evaluate impersonation username expression: %w", err)
			}
		}
		if impersonationExprs.groups != nil {
			groups, err = impersonationExprs.groups.EvaluateStringSlice(ctx, data)
			if err != nil {
				return "", nil, fmt.Errorf("failed to evaluate impersonation groups expression: %w", err)
			}
		}
		return username, groups, nil
	}

	// Build and return the provider.
	return &oauth2Provider{
		verifyAccessToken: verifyAccessToken,
		verifyToken: func(ctx context.Context, token *oauth2.Token) (string, []string, *authStorage, error) {
			idToken, ok := token.Extra("id_token").(string)
			if !ok {
				return "", nil, nil, fmt.Errorf("no id_token found in token response")
			}
			username, groups, err := verifyAccessToken(ctx, idToken)
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
		},
		config: func(ctx context.Context) (*oauth2.Config, error) {
			p, err := getProvider(ctx)
			if err != nil {
				return nil, err
			}
			return &oauth2.Config{
				Endpoint: p.Endpoint(),
				Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeOfflineAccess, "profile", "email", "groups"},
			}, nil
		},
	}, nil
}
