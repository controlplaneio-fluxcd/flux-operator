// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// AuthenticationConfigurationKind is the kind of the MCP authentication configuration object.
	AuthenticationConfigurationKind = "AuthenticationConfiguration"

	// OIDCAuthenticatorKind is the kind of the OIDC authenticator configuration object.
	OIDCAuthenticatorKind = "OIDCAuthenticator"

	// TransportBearerToken is the Bearer token transport type.
	TransportBearerToken = "BearerToken"
	// TransportBasicAuth is the Basic Authentication transport type.
	TransportBasicAuth = "BasicAuth"
	// TransportCustomHTTPHeader is the Custom HTTP Header transport type.
	TransportCustomHTTPHeader = "CustomHTTPHeader"
)

var (
	// GroupVersion is group version of the MCP authentication configuration API.
	GroupVersion = schema.GroupVersion{Group: "mcp.fluxcd.controlplane.io", Version: "v1"}
)

// ConfigObjectReference is a reference to an authentication configuration object.
type ConfigObjectReference struct {
	// Kind is the kind of the authentication configuration object.
	// +required
	Kind string `json:"kind"`

	// Name is the name of the authentication configuration object.
	// +required
	Name string `json:"name"`
}

// ConfigObject is an interface for authentication configuration objects.
type ConfigObject interface {
	GetReference() ConfigObjectReference
}

// Config is the authentication configuration object.
type Config struct {
	ConfigMeta `json:",inline"`
	Spec       ConfigSpec `json:"spec"`
}

// ConfigMeta is used for discovering the authentication configuration object type.
type ConfigMeta struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
}

// GetReference returns a ConfigObjectReference for the authentication configuration object.
func (c *ConfigMeta) GetReference() ConfigObjectReference {
	return ConfigObjectReference{
		Kind: c.Kind,
		Name: c.Name,
	}
}

// ConfigSpec is the spec field of the authentication configuration object.
type ConfigSpec struct {
	// Transports are the accepted transports for extracting credentials from a request.
	// +required
	Transports []TransportSpec `json:"transports"`

	// Authenticators are the authenticators used to validate the extracted credentials
	// and to create sessions with a username and list of groups for Kubernetes impersonation.
	// +required
	Authenticators []ConfigObjectReference `json:"authenticators"`

	// ValidateScopes indicates whether to validate that the authentication session
	// has the required scopes for the called MCP tool.
	// +optional
	ValidateScopes bool `json:"validateScopes,omitempty"`
}

// TransportSpec is the transport configuration for authentication.
type TransportSpec struct {
	// Type is the type of the transport.
	// +kubebuilder:validation:Enum=BearerToken;BasicAuth;CustomHTTPHeader
	// +required
	Type string `json:"type"`

	// Headers are the custom headers used by the CustomHTTPHeader transport.
	// +optional
	Headers *CustomHTTPHeaderSpec `json:"headers,omitempty"`
}

// CustomHTTPHeaderSpec is the configuration for the CustomHTTPHeader transport.
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

// OIDCAuthenticator is the OIDC authenticator configuration object.
type OIDCAuthenticator struct {
	ConfigMeta `json:",inline"`
	Spec       OIDCAuthenticatorSpec `json:"spec"`
}

// OIDCAuthenticatorSpec is the spec field of the OIDC authenticator configuration object.
type OIDCAuthenticatorSpec struct {
	// IssuerURL is the URL of the OIDC issuer.
	// +required
	IssuerURL string `json:"issuerURL"`

	// ClientID is the client ID of the OIDC application.
	// +required
	ClientID string `json:"clientID"`

	// Username is a CEL expression that extracts the username from the ID token claims.
	// This expression must return the type string. Defaults to "sub".
	// +optional
	Username string `json:"username,omitempty"`

	// Groups is a CEL expression that extracts the groups from the ID token claims.
	// This expression must return the type []string. Defaults to an expression that
	// returns an empty list.
	// +optional
	Groups string `json:"groups,omitempty"`

	// Scopes is a CEL expression that extracts the scopes from the ID token claims.
	// This expression must return the type []string. Defaults to an expression that
	// returns an empty list.
	// +optional
	Scopes string `json:"scopes,omitempty"`

	// Assertions is a list of CEL expressions for asserting properties of the ID token claims.
	// Each expression must return the type bool, and it must return true to consider the
	// property valid. The ID token will be considered invalid if at least one expression
	// returns false. Defaults to an empty list.
	// +optional
	Assertions []string `json:"assertions,omitempty"`
}
