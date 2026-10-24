// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/runtime/cel"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// claimsProcessorFunc defines a function type for processing claims.
type claimsProcessorFunc func(ctx context.Context, claims map[string]any) (*user.Details, error)

// newClaimsProcessor creates a new claims processor for validating and
// extracting relevant information from authentication claims.
func newClaimsProcessor(spec *fluxcdv1.ClaimsProcessorSpec) (claimsProcessorFunc, error) {
	// Build variable CEL expressions.
	type variable struct {
		name string
		expr *cel.Expression
	}
	variableExprs := make([]variable, 0, len(spec.Variables))
	for _, v := range spec.Variables {
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, err
		}
		variableExprs = append(variableExprs, variable{name: v.Name, expr: expr})
	}

	// Build validation CEL expressions.
	type validation struct {
		expr *cel.Expression
		msg  string
	}
	validationExprs := make([]validation, 0, len(spec.Validations))
	for _, v := range spec.Validations {
		expr, err := cel.NewExpression(v.Expression)
		if err != nil {
			return nil, err
		}
		validationExprs = append(validationExprs, validation{expr: expr, msg: v.Message})
	}

	// Build user info CEL expressions.
	type profile struct {
		name *cel.Expression
	}
	var profileExprs profile
	if spec.Profile != nil && spec.Profile.Name != "" {
		s := spec.Profile.Name
		expr, err := cel.NewExpression(s)
		if err != nil {
			return nil, err
		}
		profileExprs.name = expr
	}

	// Build impersonation CEL expressions.
	type impersonation struct {
		username *cel.Expression
		groups   *cel.Expression
	}
	var impersonationExprs impersonation
	if spec.Impersonation != nil && spec.Impersonation.Username != "" {
		s := spec.Impersonation.Username
		expr, err := cel.NewExpression(s)
		if err != nil {
			return nil, err
		}
		impersonationExprs.username = expr
	}
	if spec.Impersonation != nil && spec.Impersonation.Groups != "" {
		s := spec.Impersonation.Groups
		expr, err := cel.NewExpression(s)
		if err != nil {
			return nil, err
		}
		impersonationExprs.groups = expr
	}

	return func(ctx context.Context, claims map[string]any) (*user.Details, error) {
		// Extract variables from claims using CEL expressions.
		variables := map[string]any{}
		data := map[string]any{
			"claims":    claims,
			"variables": variables,
		}
		for _, x := range variableExprs {
			value, err := x.expr.Evaluate(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate variable '%s': %w", x.name, err)
			}
			variables[x.name] = value
		}

		// Validate claims and variables using CEL expressions.
		for _, v := range validationExprs {
			result, err := v.expr.EvaluateBoolean(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate validation expression: %w", err)
			}
			if !result {
				return nil, fmt.Errorf("validation failed: %s", v.msg)
			}
		}

		// Extract user info using CEL expressions.
		var profile user.Profile
		var err error
		if profileExprs.name != nil {
			profile.Name, err = profileExprs.name.EvaluateString(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate profile name expression: %w", err)
			}
		}

		// Extract impersonation info using CEL expressions.
		var imp user.Impersonation
		if impersonationExprs.username != nil {
			imp.Username, err = impersonationExprs.username.EvaluateString(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate impersonation username expression: %w", err)
			}
		}
		if impersonationExprs.groups != nil {
			imp.Groups, err = impersonationExprs.groups.EvaluateStringSlice(ctx, data)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate impersonation groups expression: %w", err)
			}
		}
		if imp.Groups == nil {
			imp.Groups = []string{}
		}

		// Sanitize and validate the extracted impersonation.
		if err := imp.SanitizeAndValidate(); err != nil {
			return nil, fmt.Errorf("impersonation validation failed: %w", err)
		}

		return &user.Details{
			Profile:       profile,
			Impersonation: imp,
		}, nil
	}, nil
}
