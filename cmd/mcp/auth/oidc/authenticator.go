// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package oidc

import (
	"context"
	"fmt"
	"net/url"

	gooidc "github.com/coreos/go-oidc/v3/oidc"

	"github.com/fluxcd/pkg/runtime/cel"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
)

// Authenticator implements the OIDC authentication mechanism.
type Authenticator struct {
	issuerURL  string
	clientID   string
	username   *cel.Expression
	groups     *cel.Expression
	scopes     *cel.Expression
	assertions []*cel.Expression
}

// New creates a new OIDC authenticator from the provided specification.
func New(spec auth.OIDCAuthenticatorSpec) (auth.Authenticator, error) {
	// Validate spec.
	issuerURL, err := url.Parse(spec.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}
	if issuerURL.Scheme != "https" {
		return nil, fmt.Errorf("issuer URL must use https scheme")
	}
	if spec.ClientID == "" {
		return nil, fmt.Errorf("client ID must be provided")
	}

	// Parse CEL expressions.
	if spec.Username == "" {
		spec.Username = "sub"
	}
	username, err := cel.NewExpression(spec.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to parse username expression '%s': %w", spec.Username, err)
	}
	if spec.Groups == "" {
		spec.Groups = "[]"
	}
	groups, err := cel.NewExpression(spec.Groups)
	if err != nil {
		return nil, fmt.Errorf("failed to parse groups expression '%s': %w", spec.Groups, err)
	}
	if spec.Scopes == "" {
		spec.Scopes = "[]"
	}
	scopes, err := cel.NewExpression(spec.Scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse scopes expression '%s': %w", spec.Scopes, err)
	}
	assertions := make([]*cel.Expression, 0, len(spec.Assertions))
	for _, assertion := range spec.Assertions {
		expr, err := cel.NewExpression(assertion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse assertion expression '%s': %w", assertion, err)
		}
		assertions = append(assertions, expr)
	}

	return &Authenticator{
		issuerURL:  spec.IssuerURL,
		clientID:   spec.ClientID,
		username:   username,
		groups:     groups,
		scopes:     scopes,
		assertions: assertions,
	}, nil
}

// Authenticate implements auth.Authenticator.
func (a *Authenticator) Authenticate(ctx context.Context, credentials auth.Credentials) (*auth.Session, error) {
	token := credentials.Token
	if token == "" {
		token = credentials.Password
	}
	provider, err := gooidc.NewProvider(ctx, a.issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}
	idToken, err := provider.VerifierContext(ctx, &gooidc.Config{ClientID: a.clientID}).Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	return a.AuthenticateToken(ctx, idToken)
}

// AuthenticateToken helps implement Authenticate.
func (a *Authenticator) AuthenticateToken(ctx context.Context, token interface{ Claims(v any) error }) (*auth.Session, error) {
	// Parse claims from the ID token.
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims from token: %w", err)
	}

	// Extract properties using CEL expressions.
	username, err := a.username.EvaluateString(ctx, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate username expression: %w", err)
	}
	groups, err := a.groups.EvaluateStringSlice(ctx, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate groups expression: %w", err)
	}
	scopes, err := a.scopes.EvaluateStringSlice(ctx, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate scopes expression: %w", err)
	}

	// Evaluate CEL assertions.
	for _, assertion := range a.assertions {
		result, err := assertion.EvaluateBoolean(ctx, claims)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate assertion expression: %w", err)
		}
		if !result {
			return nil, fmt.Errorf("assertion '%s' failed", assertion.String())
		}
	}

	return &auth.Session{
		UserName: username,
		Groups:   groups,
		Scopes:   scopes,
	}, nil
}
