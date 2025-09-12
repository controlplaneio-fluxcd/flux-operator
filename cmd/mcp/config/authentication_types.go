// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

const (
	// AuthenticationProviderOIDC is the type of the OIDC authentication provider.
	AuthenticationProviderOIDC = "OIDC"

	// AuthenticationCredentialBearerToken is the Bearer token credential type.
	AuthenticationCredentialBearerToken = "BearerToken"
	// AuthenticationCredentialBasicAuth is the Basic Authentication credential type.
	AuthenticationCredentialBasicAuth = "BasicAuth"
	// AuthenticationCredentialCustomHTTPHeader is the Custom HTTP Header credential type.
	AuthenticationCredentialCustomHTTPHeader = "CustomHTTPHeader"
)

// AuthenticationSpec holds the Flux MCP configuration.
type AuthenticationSpec struct {
	// Credentials are the accepted methods for extracting credentials from a request.
	// At least one credential must be specified.
	// +required
	Credentials []AuthenticationCredentialSpec `json:"credentials"`

	// Providers are the authentication providers used to validate the extracted credentials
	// and to create sessions with a username and list of groups for Kubernetes RBAC impersonation.
	// At least one provider must be specified.
	// +required
	Providers []AuthenticationProviderSpec `json:"providers"`
}

// AuthenticationCredentialSpec is the credential configuration for authentication.
type AuthenticationCredentialSpec struct {
	// Type is the type of the credential.
	// +kubebuilder:validation:Enum=BearerToken;BasicAuth;CustomHTTPHeader
	// +required
	Type string `json:"type"`

	// Headers are the custom headers used by the CustomHTTPHeader credential.
	// +optional
	Headers *CustomHTTPHeaderSpec `json:"headers,omitempty"`
}

// CustomHTTPHeaderSpec is the configuration for the CustomHTTPHeader credential.
type CustomHTTPHeaderSpec struct {
	// Username is the name of the HTTP header that contains the username.
	// +optional
	Username string `json:"username,omitempty"`

	// Password is the name of the HTTP header that contains the password.
	// +optional
	Password string `json:"password,omitempty"`

	// Token is the name of the HTTP header that contains the token.
	// +optional
	Token string `json:"token,omitempty"`
}

// AuthenticationProviderSpec holds the configuration for an authentication provider.
type AuthenticationProviderSpec struct {
	// Name is the name of the authentication provider.
	// +required
	Name string `json:"name"`

	// Type is the type of the authentication provider.
	// +kubebuilder:validation:Enum=OIDC
	// +required
	Type string `json:"type"`

	// IssuerURL is the URL of the OIDC issuer.
	// +required
	IssuerURL string `json:"issuerURL"`

	// Audience is the client ID of the OIDC application.
	// +required
	Audience string `json:"audience"`

	// Variables is a list of CEL expressions to extract information from the ID token claims
	// into named variables that can be reused in other expressions, e.g. "variables.username".
	// +optional
	Variables []AuthenticationProviderVariableSpec `json:"variables,omitempty"`

	// Validations is a list of CEL expressions that validate the ID token claims and extracted
	// variables. Each expression must return the type bool. If the expression evaluates to false,
	// the message is returned as an error.
	// +optional
	Validations []AuthenticationProviderValidationSpec `json:"validations,omitempty"`

	// Impersonation is a pair of CEL expressions that extract the username and groups
	// from the ID token claims and extracted variables for Kubernetes RBAC impersonation.
	// The username expression must return the type string, while the groups expression
	// must return the type []string.
	// +optional
	Impersonation *AuthenticationProviderImpersonationSpec `json:"impersonation,omitempty"`

	// Scopes is the configuration for the validation of scopes.
	// +optional
	Scopes *AuthenticationProviderScopesSpec `json:"scopes,omitempty"`
}

// AuthenticationProviderVariableSpec holds the configuration for a variable used in CEL expressions.
type AuthenticationProviderVariableSpec struct {
	// Name is the name of the variable.
	// +required
	Name string `json:"name"`

	// Expression is the CEL expression that defines the variable.
	// +required
	Expression string `json:"expression"`
}

// AuthenticationProviderValidationSpec holds the configuration for a validation used in CEL expressions.
type AuthenticationProviderValidationSpec struct {
	// Expression is the CEL expression that defines the validation.
	// The expression must return the type bool.
	// +required
	Expression string `json:"expression"`

	// Message is the error message returned if the validation fails.
	// +required
	Message string `json:"message"`
}

// AuthenticationProviderImpersonationSpec holds CEL expressions for Kubernetes RBAC impersonation information.
type AuthenticationProviderImpersonationSpec struct {
	// Username is a CEL expression that extracts the username from the ID token claims
	// and extracted variables. This expression must return the type string.
	// +optional
	Username string `json:"username,omitempty"`

	// Groups is a CEL expression that extracts the groups from the ID token claims
	// and extracted variables. This expression must return the type []string.
	// +optional
	Groups string `json:"groups,omitempty"`
}

// AuthenticationProviderScopesSpec is the configuration for the validation of scopes.
type AuthenticationProviderScopesSpec struct {
	// Expression is a CEL expression that extracts the scopes from the ID token claims
	// and extracted variables. This expression must return the type []string.
	// +required
	Expression string `json:"expression"`
}
