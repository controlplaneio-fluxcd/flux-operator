// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package factory

import (
	"fmt"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/oidc"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/config"
)

// New creates a new authenticator function from the provided authentication configuration.
func New(conf config.AuthenticationSpec) (auth.AuthenticatorFunc, error) {
	// Build all credentials.
	var credentials auth.CredentialSet
	for i, ts := range conf.Credentials {
		switch ts.Type {
		case config.AuthenticationCredentialBearerToken:
			credentials = append(credentials, auth.BearerTokenCredential{})
		case config.AuthenticationCredentialBasicAuth:
			credentials = append(credentials, auth.BasicAuthCredential{})
		case config.AuthenticationCredentialCustomHTTPHeader:
			if ts.Headers == nil {
				return nil, fmt.Errorf("missing headers configuration for CustomHTTPHeader credential at index %d", i)
			}
			credentials = append(credentials, &auth.CustomHTTPHeaderCredential{CustomHTTPHeaderSpec: *ts.Headers})
		default:
			return nil, fmt.Errorf("unsupported credential type '%s' in the authentication configuration", ts.Type)
		}
	}
	if len(credentials) == 0 {
		return nil, fmt.Errorf("no credentials found in the authentication configuration")
	}

	// Build all providers.
	var providers auth.ProviderSet
	providerNames := make(map[string]struct{})
	for i, providerConf := range conf.Providers {
		if providerConf.Type == "" {
			return nil, fmt.Errorf("missing type in provider reference at index %d", i)
		}

		if providerConf.Name == "" {
			return nil, fmt.Errorf("missing name in provider reference at index %d", i)
		}

		if _, exists := providerNames[providerConf.Name]; exists {
			return nil, fmt.Errorf("duplicate provider name '%s' in the authentication configuration", providerConf.Name)
		}
		providerNames[providerConf.Name] = struct{}{}

		switch providerConf.Type {
		case config.AuthenticationProviderOIDC:
			provider, err := oidc.New(providerConf)
			if err != nil {
				return nil, fmt.Errorf("failed to create OIDC provider %s: %w", providerConf.Name, err)
			}
			providers = append(providers, provider)
		default:
			return nil, fmt.Errorf("unknown provider type '%s' in the authentication configuration", providerConf.Type)
		}
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers found in the authentication configuration")
	}

	return auth.New(credentials, providers), nil
}
