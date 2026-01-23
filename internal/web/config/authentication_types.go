// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/runtime/cel"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/user"
)

const (
	// AuthenticationTypeAnonymous is the name of the Anonymous authentication type.
	AuthenticationTypeAnonymous = "Anonymous"

	// AuthenticationTypeOAuth2 is the name of the OAuth2 authentication type.
	AuthenticationTypeOAuth2 = "OAuth2"

	// OAuth2ProviderOIDC is the name of the OIDC OAuth2 provider.
	OAuth2ProviderOIDC = "OIDC"
)

var (
	// AllAuthenticationTypes lists all possible authentication types.
	AllAuthenticationTypes = []string{
		AuthenticationTypeAnonymous,
		AuthenticationTypeOAuth2,
	}
)

// AuthenticationSpec holds the Flux Status Page configuration.
type AuthenticationSpec struct {
	// Type is the authentication type.
	// +kubebuilder:validation:Enum=Anonymous;OAuth2
	// +required
	Type string `json:"type"`

	// Anonymous holds the Anonymous authentication configuration.
	// +optional
	Anonymous *AnonymousAuthenticationSpec `json:"anonymous"`

	// OAuth2 holds the OAuth2 authentication configuration.
	// +optional
	OAuth2 *OAuth2AuthenticationSpec `json:"oauth2"`

	// SessionDuration is the duration of the user session.
	// Defaults to one week.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="^([0-9]+(\\.[0-9]+)?(ms|s|m|h))+$"
	// +optional
	SessionDuration *metav1.Duration `json:"sessionDuration"`

	// UserCacheSize is the size of the user cache in number of users.
	// Defaults to 100.
	// +optional
	UserCacheSize int `json:"userCacheSize"`
}

// AuthenticationConfiguration is an interface for different authentication configurations.
type AuthenticationConfiguration interface {
	Configured() bool
	Validate() error
}

// Validate validates the AuthenticationSpec configuration.
func (a *AuthenticationSpec) Validate() error {
	authConfigs := make(map[string]AuthenticationConfiguration)

	// For each authentication type, add it to the map.
	authConfigs[AuthenticationTypeAnonymous] = a.Anonymous
	authConfigs[AuthenticationTypeOAuth2] = a.OAuth2

	// Validate that the specified authentication type is valid and configured.
	switch c, ok := authConfigs[a.Type]; {
	case !ok:
		return fmt.Errorf("invalid authentication type '%s'", a.Type)
	case !c.Configured():
		return fmt.Errorf("authentication type '%s' is not configured", a.Type)
	}

	// Validate configurations.
	authConfigTypes := make([]string, 0, len(authConfigs))
	for authType, c := range authConfigs {
		if !c.Configured() {
			continue
		}
		if err := c.Validate(); err != nil {
			return fmt.Errorf("invalid %s authentication configuration: %w", authType, err)
		}
		authConfigTypes = append(authConfigTypes, authType)
	}

	// Validate that only a single authentication configuration is provided.
	if len(authConfigTypes) > 1 {
		return fmt.Errorf("multiple authentication configurations found, only one is allowed: [%s]",
			strings.Join(authConfigTypes, ", "))
	}

	return nil
}

// ApplyDefaults applies default values to the AuthenticationSpec.
func (a *AuthenticationSpec) ApplyDefaults() {
	if a == nil {
		return
	}

	a.Anonymous.ApplyDefaults()
	a.OAuth2.ApplyDefaults()

	if a.SessionDuration == nil || a.SessionDuration.Duration <= 0 {
		a.SessionDuration = &metav1.Duration{Duration: 7 * 24 * time.Hour}
	}

	if a.UserCacheSize <= 0 {
		a.UserCacheSize = 100
	}
}

// AnonymousAuthenticationSpec holds the Anonymous authentication configuration.
// At least one of the fields must be set.
type AnonymousAuthenticationSpec struct {
	// Username is used for Kubernetes User RBAC impersonation.
	// +optional
	Username string `json:"username"`

	// Groups is used for Kubernetes Group RBAC impersonation.
	// +optional
	Groups []string `json:"groups"`
}

// Configured checks if the AnonymousAuthenticationSpec is configured.
func (a *AnonymousAuthenticationSpec) Configured() bool { return a != nil }

// Validate validates the AnonymousAuthenticationSpec configuration.
func (a *AnonymousAuthenticationSpec) Validate() error {
	imp := (user.Impersonation)(*a)
	if err := imp.SanitizeAndValidate(); err != nil {
		return fmt.Errorf("invalid anonymous authentication impersonation: %w", err)
	}
	*a = (AnonymousAuthenticationSpec)(imp)
	return nil
}

// ApplyDefaults applies default values to the AnonymousAuthenticationSpec.
func (a *AnonymousAuthenticationSpec) ApplyDefaults() {
	if a == nil {
		return
	}
	if a.Groups == nil {
		a.Groups = []string{}
	}
}

// OAuth2AuthenticationSpec holds the OAuth2 authentication configuration.
type OAuth2AuthenticationSpec struct {
	// Provider is the OAuth2 provider name.
	// +kubebuilder:validation:Enum=OIDC
	// +required
	Provider string `json:"provider"`

	// ClientID is the OAuth2 client identifier.
	// +required
	ClientID string `json:"clientID"`

	// ClientSecret is the OAuth2 client secret.
	// +required
	ClientSecret string `json:"clientSecret"`

	// Scopes is the list of OAuth2 scopes to request.
	// Each provider may have different default scopes.
	// +optional
	Scopes []string `json:"scopes"`

	// IssuerURL is used for OIDC provider discovery.
	// Required for the OIDC provider.
	// Used only by the OIDC provider.
	// +optional
	IssuerURL string `json:"issuerURL"`

	// ClaimsProcessorSpec holds the configuration for processing claims with CEL expressions.
	// Used only by the OIDC provider.
	// +optional
	ClaimsProcessorSpec `json:",inline"`
}

// Configured checks if the OAuth2AuthenticationSpec is configured.
func (o *OAuth2AuthenticationSpec) Configured() bool { return o != nil }

// Validate validates the OAuth2AuthenticationSpec configuration.
func (o *OAuth2AuthenticationSpec) Validate() error {
	if o.ClientID == "" {
		return fmt.Errorf("clientID must be set for OAuth2 authentication")
	}

	if o.ClientSecret == "" {
		return fmt.Errorf("clientSecret must be set for OAuth2 authentication")
	}

	switch o.Provider {
	case OAuth2ProviderOIDC:
		if o.IssuerURL == "" {
			return fmt.Errorf("issuerURL must be set for the OIDC OAuth2 provider")
		}
		if _, err := url.Parse(o.IssuerURL); err != nil {
			return fmt.Errorf("issuerURL is not a valid URL: %w", err)
		}

		if err := o.ClaimsProcessorSpec.Validate(); err != nil {
			return err
		}
	default:
		// TODO: when introducing more providers, validate that the OIDC-only fields are not set.
		return fmt.Errorf("invalid OAuth2 provider: '%s'", o.Provider)
	}

	return nil
}

// ApplyDefaults applies default values to the OAuth2AuthenticationSpec.
func (o *OAuth2AuthenticationSpec) ApplyDefaults() {
	if o == nil {
		return
	}

	switch o.Provider {
	case OAuth2ProviderOIDC:
		o.ClaimsProcessorSpec.ApplyDefaults()
	}
}

// ClaimsProcessorSpec holds the configuration for processing claims with CEL expressions.
type ClaimsProcessorSpec struct {
	// Variables is a list of CEL expressions to extract information from the ID token claims
	// into named variables that can be reused in other expressions, e.g. "variables.username".
	// +optional
	Variables []VariableSpec `json:"variables"`

	// Validations is a list of CEL expressions that validate the ID token claims and extracted
	// variables. Each expression must return the type bool. If the expression evaluates to false,
	// the message is returned as an error.
	// +optional
	Validations []ValidationSpec `json:"validations"`

	// Profile contains CEL expressions to extract user profile information from the ID token
	// claims and extracted variables for populating the user profile.
	// Defaults to ProfileSpec{
	//   Name:  "has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')",
	// }
	// +optional
	Profile *ProfileSpec `json:"profile"`

	// Impersonation is a pair of CEL expressions that extract the username and groups
	// from the ID token claims and extracted variables for Kubernetes RBAC impersonation.
	// The username expression must return the type string, while the groups expression
	// must return the type []string.
	// Defaults to ImpersonationSpec{
	//   Username: "has(claims.email) ? claims.email : ''",
	//   Groups:   "has(claims.groups) ? claims.groups : []",
	// }
	// +optional
	Impersonation *ImpersonationSpec `json:"impersonation"`
}

// Validate validates the ClaimsProcessorSpec configuration.
func (c *ClaimsProcessorSpec) Validate() error {
	for i, v := range c.Variables {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid variable[%d]: %w", i, err)
		}
	}
	for i, v := range c.Validations {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("invalid validation[%d]: %w", i, err)
		}
	}
	if err := c.Profile.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}
	if err := c.Impersonation.Validate(); err != nil {
		return fmt.Errorf("invalid impersonation: %w", err)
	}
	return nil
}

// ApplyDefaults applies default values to the ClaimsProcessorSpec.
func (c *ClaimsProcessorSpec) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.Profile == nil {
		c.Profile = &ProfileSpec{}
	}
	c.Profile.ApplyDefaults()
	if c.Impersonation == nil {
		c.Impersonation = &ImpersonationSpec{}
	}
	c.Impersonation.ApplyDefaults()
}

// VariableSpec holds the configuration for extracting a variable with a CEL expression.
type VariableSpec struct {
	// Name is the name of the variable.
	// +required
	Name string `json:"name"`

	// Expression is the CEL expression that defines the variable.
	// +required
	Expression string `json:"expression"`
}

// Validate validates the VariableSpec configuration.
func (v *VariableSpec) Validate() error {
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

// ValidationSpec holds the configuration for a validation used in CEL expressions.
type ValidationSpec struct {
	// Expression is the CEL expression that defines the validation.
	// The expression must return the type bool.
	// +required
	Expression string `json:"expression"`

	// Message is the error message returned if the validation fails.
	// +required
	Message string `json:"message"`
}

// Validate validates the ValidationSpec configuration.
func (v *ValidationSpec) Validate() error {
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

// ProfileSpec holds CEL expressions for extracting user profile information.
type ProfileSpec struct {
	// Name is a CEL expression that extracts the user's full name from the ID token claims
	// and extracted variables. This expression must return the type string.
	// Defaults to "has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')".
	// +optional
	Name string `json:"name"`
}

// Validate validates the ProfileSpec configuration.
func (u *ProfileSpec) Validate() error {
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

// ApplyDefaults applies default values to the ProfileSpec.
func (u *ProfileSpec) ApplyDefaults() {
	if u == nil {
		return
	}
	if u.Name == "" {
		u.Name = "has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')"
	}
}

// ImpersonationSpec holds CEL expressions for extracting Kubernetes RBAC impersonation information.
// At least one of the fields must be set.
type ImpersonationSpec struct {
	// Username is a CEL expression that extracts the username from the ID token claims
	// and extracted variables. This expression must return the type string.
	// Defaults to "has(claims.email) ? claims.email : ''".
	// +optional
	Username string `json:"username"`

	// Groups is a CEL expression that extracts the groups from the ID token claims
	// and extracted variables. This expression must return the type []string.
	// Defaults to "has(claims.groups) ? claims.groups : []".
	// +optional
	Groups string `json:"groups"`
}

// Validate validates the ImpersonationSpec configuration.
func (i *ImpersonationSpec) Validate() error {
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

// ApplyDefaults applies default values to the ImpersonationSpec.
func (i *ImpersonationSpec) ApplyDefaults() {
	if i == nil {
		return
	}
	if i.Username == "" {
		i.Username = "has(claims.email) ? claims.email : ''"
	}
	if i.Groups == "" {
		i.Groups = "has(claims.groups) ? claims.groups : []"
	}
}
