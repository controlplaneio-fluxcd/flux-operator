// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/fluxcd/pkg/runtime/cel"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

// ValidateWebConfig validates the WebConfig configuration.
func ValidateWebConfig(c *fluxcdv1.WebConfig) error {
	if c.GroupVersionKind() != fluxcdv1.WebConfigGroupVersion.WithKind(fluxcdv1.WebConfigKind) {
		return fmt.Errorf("expected apiVersion '%s' and kind '%s', got '%s' and '%s'",
			fluxcdv1.WebConfigGroupVersion.String(), fluxcdv1.WebConfigKind, c.APIVersion, c.Kind)
	}
	return ValidateWebConfigSpec(&c.Spec)
}

// ValidateWebConfigSpec validates the WebConfigSpec configuration.
func ValidateWebConfigSpec(c *fluxcdv1.WebConfigSpec) error {
	baseURLRequired := c.Authentication != nil && c.Authentication.Type == fluxcdv1.AuthenticationTypeOAuth2
	if baseURLRequired && c.BaseURL == "" {
		return fmt.Errorf("baseURL must be set when OAuth2 authentication is configured")
	}
	if c.BaseURL != "" {
		if _, err := url.Parse(c.BaseURL); err != nil {
			return fmt.Errorf("invalid baseURL: %w", err)
		}
	}

	if c.UserActions != nil {
		if err := ValidateUserActionsSpec(c.UserActions); err != nil {
			return fmt.Errorf("invalid user actions configuration: %w", err)
		}
	}

	if c.Authentication != nil {
		if err := ValidateAuthenticationSpec(c.Authentication); err != nil {
			return fmt.Errorf("invalid authentication configuration: %w", err)
		}
	}

	return nil
}

// ValidateAuthenticationSpec validates the AuthenticationSpec configuration.
func ValidateAuthenticationSpec(a *fluxcdv1.AuthenticationSpec) error {
	// Validate that the specified authentication type is valid and configured.
	switch a.Type {
	case fluxcdv1.AuthenticationTypeAnonymous:
		if !a.Anonymous.Configured() {
			return fmt.Errorf("authentication type '%s' is not configured", a.Type)
		}
	case fluxcdv1.AuthenticationTypeOAuth2:
		if !a.OAuth2.Configured() {
			return fmt.Errorf("authentication type '%s' is not configured", a.Type)
		}
	default:
		return fmt.Errorf("invalid authentication type '%s'", a.Type)
	}

	// Validate configurations and count how many are configured.
	var authConfigTypes []string

	if a.Anonymous.Configured() {
		if err := ValidateAnonymousAuthenticationSpec(a.Anonymous); err != nil {
			return fmt.Errorf("invalid %s authentication configuration: %w",
				fluxcdv1.AuthenticationTypeAnonymous, err)
		}
		authConfigTypes = append(authConfigTypes, fluxcdv1.AuthenticationTypeAnonymous)
	}

	if a.OAuth2.Configured() {
		if err := ValidateOAuth2AuthenticationSpec(a.OAuth2); err != nil {
			return fmt.Errorf("invalid %s authentication configuration: %w",
				fluxcdv1.AuthenticationTypeOAuth2, err)
		}
		authConfigTypes = append(authConfigTypes, fluxcdv1.AuthenticationTypeOAuth2)
	}

	// Validate that only a single authentication configuration is provided.
	if len(authConfigTypes) > 1 {
		return fmt.Errorf("multiple authentication configurations found, only one is allowed: [%s]",
			strings.Join(authConfigTypes, ", "))
	}

	return nil
}

// ValidateAnonymousAuthenticationSpec validates the AnonymousAuthenticationSpec configuration.
func ValidateAnonymousAuthenticationSpec(a *fluxcdv1.AnonymousAuthenticationSpec) error {
	imp := user.Impersonation{
		Username: a.Username,
		Groups:   a.Groups,
	}
	if err := imp.SanitizeAndValidate(); err != nil {
		return fmt.Errorf("invalid anonymous authentication impersonation: %w", err)
	}
	// Update the spec with sanitized values
	a.Username = imp.Username
	a.Groups = imp.Groups
	return nil
}

// ValidateOAuth2AuthenticationSpec validates the OAuth2AuthenticationSpec configuration.
func ValidateOAuth2AuthenticationSpec(o *fluxcdv1.OAuth2AuthenticationSpec) error {
	if o.ClientID == "" {
		return fmt.Errorf("clientID must be set for OAuth2 authentication")
	}

	if o.ClientSecret == "" {
		return fmt.Errorf("clientSecret must be set for OAuth2 authentication")
	}

	switch o.Provider {
	case fluxcdv1.OAuth2ProviderOIDC:
		if o.IssuerURL == "" {
			return fmt.Errorf("issuerURL must be set for the OIDC OAuth2 provider")
		}
		if _, err := url.Parse(o.IssuerURL); err != nil {
			return fmt.Errorf("issuerURL is not a valid URL: %w", err)
		}

		if err := ValidateClaimsProcessorSpec(&o.ClaimsProcessorSpec); err != nil {
			return err
		}
	default:
		// TODO: when introducing more providers, validate that the OIDC-only fields are not set.
		return fmt.Errorf("invalid OAuth2 provider: '%s'", o.Provider)
	}

	return nil
}

// ValidateClaimsProcessorSpec validates the ClaimsProcessorSpec configuration.
func ValidateClaimsProcessorSpec(c *fluxcdv1.ClaimsProcessorSpec) error {
	for i, v := range c.Variables {
		if err := ValidateVariableSpec(&v); err != nil {
			return fmt.Errorf("invalid variable[%d]: %w", i, err)
		}
	}
	for i, v := range c.Validations {
		if err := ValidateValidationSpec(&v); err != nil {
			return fmt.Errorf("invalid validation[%d]: %w", i, err)
		}
	}
	if err := ValidateProfileSpec(c.Profile); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}
	if err := ValidateImpersonationSpec(c.Impersonation); err != nil {
		return fmt.Errorf("invalid impersonation: %w", err)
	}
	return nil
}

// ValidateVariableSpec validates the VariableSpec configuration.
func ValidateVariableSpec(v *fluxcdv1.VariableSpec) error {
	if v.Name == "" {
		return fmt.Errorf("variable name must be provided")
	}
	if v.Expression == "" {
		return fmt.Errorf("variable expression must be provided")
	}
	if _, err := cel.NewExpression(v.Expression); err != nil {
		return fmt.Errorf("failed to parse variable '%s' CEL expression '%s': %w",
			v.Name, v.Expression, err)
	}
	return nil
}

// ValidateValidationSpec validates the ValidationSpec configuration.
func ValidateValidationSpec(v *fluxcdv1.ValidationSpec) error {
	if v.Expression == "" {
		return fmt.Errorf("validation expression must be provided")
	}
	if v.Message == "" {
		return fmt.Errorf("validation message must be provided")
	}
	if _, err := cel.NewExpression(v.Expression); err != nil {
		return fmt.Errorf("failed to parse validation CEL expression '%s': %w",
			v.Expression, err)
	}
	return nil
}

// ValidateProfileSpec validates the ProfileSpec configuration.
func ValidateProfileSpec(u *fluxcdv1.ProfileSpec) error {
	if u == nil {
		return nil
	}
	if u.Name != "" {
		if _, err := cel.NewExpression(u.Name); err != nil {
			return fmt.Errorf("failed to parse name expression '%s' for user profile: %w",
				u.Name, err)
		}
	}
	return nil
}

// ValidateImpersonationSpec validates the ImpersonationSpec configuration.
func ValidateImpersonationSpec(i *fluxcdv1.ImpersonationSpec) error {
	if i == nil {
		return nil
	}
	if i.Username == "" && i.Groups == "" {
		return fmt.Errorf("impersonation must have at least one of username or groups expressions")
	}
	if i.Username != "" {
		if _, err := cel.NewExpression(i.Username); err != nil {
			return fmt.Errorf("failed to parse username expression '%s' for impersonation: %w",
				i.Username, err)
		}
	}
	if i.Groups != "" {
		if _, err := cel.NewExpression(i.Groups); err != nil {
			return fmt.Errorf("failed to parse groups expression '%s' for impersonation: %w",
				i.Groups, err)
		}
	}
	return nil
}

// ValidateUserActionsSpec validates the UserActionsSpec configuration.
func ValidateUserActionsSpec(u *fluxcdv1.UserActionsSpec) error {
	auditedActions := make(map[string]struct{})
	for _, action := range u.Audit {
		if _, exists := auditedActions[action]; exists {
			return fmt.Errorf("duplicate audit action: '%s'", action)
		}
		if !slices.Contains(fluxcdv1.AllUserActions, action) && action != "*" {
			return fmt.Errorf("invalid audit action: '%s'", action)
		}
		auditedActions[action] = struct{}{}
	}
	if _, exists := auditedActions["*"]; exists && len(auditedActions) > 1 {
		return fmt.Errorf("audit action '*' cannot be combined with other actions")
	}
	return nil
}
