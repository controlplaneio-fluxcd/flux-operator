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
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

// Provider implements the OIDC authentication mechanism.
type Provider struct {
	issuerURL     string
	clientID      string
	variables     []variable
	validations   []validation
	impersonation *kubernetesImpersonation
	scopes        *scopesConfig
}

type variable struct {
	name string
	expr *cel.Expression
}

type validation struct {
	expr *cel.Expression
	msg  string
}

type kubernetesImpersonation struct {
	username *cel.Expression
	groups   *cel.Expression
}

type scopesConfig struct {
	expr *cel.Expression
}

// New creates a new OIDC authentication provider from the given specification.
func New(spec config.AuthenticationProviderSpec) (auth.Provider, error) {
	// Validate spec.
	issuerURL, err := url.Parse(spec.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}
	if issuerURL.Scheme != "https" {
		return nil, fmt.Errorf("issuer URL must use https scheme")
	}
	if spec.Audience == "" {
		return nil, fmt.Errorf("audience must be provided")
	}
	clientID := spec.Audience

	// Parse variables.
	variables := make([]variable, 0, len(spec.Variables))
	for _, v := range spec.Variables {
		if v.Name == "" {
			return nil, fmt.Errorf("variable name must be provided")
		}
		if v.Expression == "" {
			return nil, fmt.Errorf("variable expression must be provided")
		}
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to parse variable '%s' CEL expression '%s': %w",
				v.Name, v.Expression, err)
		}
		variables = append(variables, variable{name: v.Name, expr: expr})
	}

	// Parse validations.
	validations := make([]validation, 0, len(spec.Validations))
	for _, v := range spec.Validations {
		if v.Expression == "" {
			return nil, fmt.Errorf("validation expression must be provided")
		}
		if v.Message == "" {
			return nil, fmt.Errorf("validation message must be provided")
		}
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to parse validation CEL expression '%s': %w",
				v.Expression, err)
		}
		validations = append(validations, validation{expr: expr, msg: v.Message})
	}

	// Parse impersonation.
	var impersonation *kubernetesImpersonation
	if spec.Impersonation != nil {
		if spec.Impersonation.Username == "" && spec.Impersonation.Groups == "" {
			return nil, fmt.Errorf("impersonation must have at least one of username or groups expressions")
		}
		impersonation = &kubernetesImpersonation{}
		if spec.Impersonation.Username != "" {
			expr, err := cel.NewExpression(spec.Impersonation.Username)
			if err != nil {
				return nil, fmt.Errorf("failed to parse impersonation username expression '%s': %w",
					spec.Impersonation.Username, err)
			}
			impersonation.username = expr
		}
		if spec.Impersonation.Groups != "" {
			expr, err := cel.NewExpression(spec.Impersonation.Groups)
			if err != nil {
				return nil, fmt.Errorf("failed to parse impersonation groups expression '%s': %w",
					spec.Impersonation.Groups, err)
			}
			impersonation.groups = expr
		}
	}

	// Parse scopes.
	var scopes *scopesConfig
	if spec.Scopes != nil {
		if spec.Scopes.Expression == "" {
			return nil, fmt.Errorf("scopes expression must be provided")
		}
		scopes = &scopesConfig{}
		expr, err := cel.NewExpression(spec.Scopes.Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to parse scopes expression '%s': %w",
				spec.Scopes.Expression, err)
		}
		scopes.expr = expr
	}

	return &Provider{
		issuerURL:     issuerURL.String(),
		clientID:      clientID,
		variables:     variables,
		validations:   validations,
		impersonation: impersonation,
		scopes:        scopes,
	}, nil
}

// Authenticate implements auth.Provider.
func (a *Provider) Authenticate(ctx context.Context, credentials auth.ExtractedCredentials) (*auth.Session, error) {
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
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims from token: %w", err)
	}
	return a.AuthenticateClaims(ctx, claims)
}

// AuthenticateClaims helps implement Authenticate.
func (a *Provider) AuthenticateClaims(ctx context.Context, claims map[string]any) (*auth.Session, error) {
	// Extract variables from claims using CEL expressions.
	variables := map[string]any{}
	data := map[string]any{
		"claims":    claims,
		"variables": variables,
	}
	for _, x := range a.variables {
		value, err := x.expr.Evaluate(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate variable '%s': %w", x.name, err)
		}
		variables[x.name] = value
	}

	// Validate claims and variables using CEL expressions.
	for _, v := range a.validations {
		result, err := v.expr.EvaluateBoolean(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate validation expression: %w", err)
		}
		if !result {
			return nil, fmt.Errorf("validation failed: %s", v.msg)
		}
	}

	var sess auth.Session

	// Extract impersonation info using CEL expressions.
	if a.impersonation != nil {
		if a.impersonation.username != nil {
			username, err := a.impersonation.username.EvaluateString(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate impersonation username expression: %w", err)
			}
			sess.UserName = username
		}
		if a.impersonation.groups != nil {
			groups, err := a.impersonation.groups.EvaluateStringSlice(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate impersonation groups expression: %w", err)
			}
			sess.Groups = groups
		}
	}

	// Extract scopes using CEL expression.
	if a.scopes != nil {
		scopes, err := a.scopes.expr.EvaluateStringSlice(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate scopes expression: %w", err)
		}
		sess.Scopes = scopes
		if len(sess.Scopes) == 0 {
			sess.Scopes = []string{}
		}
	}

	return &sess, nil
}
