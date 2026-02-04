// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// WebConfigKind is the kind of the Flux Status Page configuration API.
	WebConfigKind = "Config"

	// AuthenticationTypeAnonymous is the name of the Anonymous authentication type.
	AuthenticationTypeAnonymous = "Anonymous"

	// AuthenticationTypeOAuth2 is the name of the OAuth2 authentication type.
	AuthenticationTypeOAuth2 = "OAuth2"

	// OAuth2ProviderOIDC is the name of the OIDC OAuth2 provider.
	OAuth2ProviderOIDC = "OIDC"

	// UserActionReconcile is the reconcile user action for Flux resources.
	UserActionReconcile = "reconcile"

	// UserActionSuspend is the suspend user action for Flux resources.
	UserActionSuspend = "suspend"

	// UserActionResume is the resume user action for Flux resources.
	UserActionResume = "resume"

	// UserActionDownload is the download user action for Flux artifacts.
	UserActionDownload = "download"

	// UserActionRestart is the restart user action for workloads
	// (Deployments, StatefulSets, DaemonSets).
	UserActionRestart = "restart"
)

var (
	// AllAuthenticationTypes lists all possible authentication types.
	AllAuthenticationTypes = []string{
		AuthenticationTypeAnonymous,
		AuthenticationTypeOAuth2,
	}

	// AllUserActions lists all possible user actions.
	AllUserActions = []string{
		UserActionReconcile,
		UserActionSuspend,
		UserActionResume,
		UserActionDownload,
		UserActionRestart,
	}
)

// WebConfig is the Flux Status Page configuration.
type WebConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Spec holds the Flux Status Page configuration.
	// +required
	Spec WebConfigSpec `json:"spec"`
}

// WebConfigSpec holds the Flux Status Page configuration.
type WebConfigSpec struct {
	// Version is a unique identifier for the configuration.
	// This field is set internally when the configuration
	// is loaded and is not part of the API.
	// +required
	Version string `json:"-"`

	// BaseURL is the base URL for constructing the Flux Status Page URLs.
	// Some features may require this to be set.
	// +optional
	BaseURL string `json:"baseURL"`

	// Insecure indicates whether to use insecure settings across the web application.
	// +optional
	Insecure bool `json:"insecure"`

	// UserActions holds the user actions configuration. Defaults to enabling all actions if not set.
	// Note that, by default, actions are only available when authentication is configured with the
	// OAuth2 type.
	// +optional
	UserActions *UserActionsSpec `json:"userActions"`

	// Authentication holds the authentication configuration.
	// If Authentication.Type is set to OAuth2, BaseURL must be set.
	// +optional
	Authentication *AuthenticationSpec `json:"authentication"`
}

// UserActionsEnabled checks if user actions are enabled.
func (c *WebConfigSpec) UserActionsEnabled() bool {
	return c != nil && c.Authentication != nil
}

// AuthenticationSpec holds the Flux Status Page authentication configuration.
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

// VariableSpec holds the configuration for extracting a variable with a CEL expression.
type VariableSpec struct {
	// Name is the name of the variable.
	// +required
	Name string `json:"name"`

	// Expression is the CEL expression that defines the variable.
	// +required
	Expression string `json:"expression"`
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

// ProfileSpec holds CEL expressions for extracting user profile information.
type ProfileSpec struct {
	// Name is a CEL expression that extracts the user's full name from the ID token claims
	// and extracted variables. This expression must return the type string.
	// Defaults to "has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')".
	// +optional
	Name string `json:"name"`
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

// UserActionsSpec holds the actions configuration.
type UserActionsSpec struct {
	// Audit is a list of actions to be audited.
	// If the field is empty or omitted, no actions are audited.
	// The special value ["*"] can be used to audit all actions.
	// +optional
	Audit []string `json:"audit"`
}
