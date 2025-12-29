// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"fmt"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConfigKind is the kind of the Flux Status Page configuration API.
	ConfigKind = "Config"
)

// Config is the Flux Status Page configuration.
type Config struct {
	metav1.TypeMeta `json:",inline"`

	// Spec holds the Flux Status Page configuration.
	Spec ConfigSpec `json:"spec"`
}

// Validate validates the Config configuration.
func (c Config) Validate() error {
	if c.GroupVersionKind() != GroupVersion.WithKind(ConfigKind) {
		return fmt.Errorf("expected apiVersion '%s' and kind '%s', got '%s' and '%s'",
			GroupVersion.String(), ConfigKind, c.APIVersion, c.Kind)
	}
	return c.Spec.Validate()
}

// ConfigSpec holds the Flux Status Page configuration.
type ConfigSpec struct {
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
	Insecure bool `json:"insecure,omitempty"`

	// UserActions holds the user actions configuration. Defaults to enabling all actions if not set.
	// Note that actions are only available when authentication is configured with the OAuth2 type.
	// +optional
	UserActions *UserActionsSpec `json:"userActions,omitempty"`

	// Authentication holds the authentication configuration.
	// If Authentication.Type is set to OAuth2, BaseURL must be set.
	// +optional
	Authentication *AuthenticationSpec `json:"authentication,omitempty"`
}

// Validate validates the ConfigSpec configuration.
func (c ConfigSpec) Validate() error {
	baseURLRequired := c.Authentication != nil && c.Authentication.Type == AuthenticationTypeOAuth2
	if baseURLRequired && c.BaseURL == "" {
		return fmt.Errorf("baseURL must be set when OAuth2 authentication is configured")
	}
	if c.BaseURL != "" {
		if _, err := url.Parse(c.BaseURL); err != nil {
			return fmt.Errorf("invalid baseURL: %w", err)
		}
	}

	if c.UserActions != nil {
		if err := c.UserActions.Validate(); err != nil {
			return fmt.Errorf("invalid user actions configuration: %w", err)
		}
	}

	if c.Authentication != nil {
		if err := c.Authentication.Validate(); err != nil {
			return fmt.Errorf("invalid authentication configuration: %w", err)
		}
	}

	return nil
}

// ApplyDefaults applies default values to the ConfigSpec.
func (c *ConfigSpec) ApplyDefaults() {
	c.UserActions.ApplyDefaults()
	c.Authentication.ApplyDefaults()
}
