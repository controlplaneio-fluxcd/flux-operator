// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package factory

import (
	"bufio"
	"fmt"
	"io"

	"github.com/mark3labs/mcp-go/server"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth/oidc"
)

// New creates a new authentication middleware from the provided configuration file path.
func New(file io.Reader) (server.ToolHandlerMiddleware, error) {
	reader := utilyaml.NewYAMLReader(bufio.NewReader(file))

	var conf *auth.Config
	confObjects := make(map[auth.ConfigObjectReference]auth.ConfigObject)

	// Unmarshal all objects.
	for {
		b, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read auth config file: %w", err)
		}

		var confMeta auth.ConfigMeta
		if err := yaml.Unmarshal(b, &confMeta); err != nil {
			return nil, fmt.Errorf("failed to unmarshal auth config object from YAML: %w", err)
		}

		if confMeta.APIVersion != auth.GroupVersion.String() {
			return nil, fmt.Errorf("unsupported apiVersion '%s' in the auth config file", confMeta.APIVersion)
		}

		if confMeta.GetName() == "" {
			return nil, fmt.Errorf("missing metadata.name in the auth config file")
		}

		switch confMeta.Kind {
		case auth.AuthenticationConfigurationKind:
			if conf != nil {
				return nil, fmt.Errorf("multiple authentication configuration objects found in the auth config file")
			}
			conf = &auth.Config{}
			if err := yaml.Unmarshal(b, conf); err != nil {
				return nil, fmt.Errorf("failed to unmarshal authentication configuration object from YAML: %w", err)
			}
		case auth.OIDCAuthenticatorKind:
			var obj auth.OIDCAuthenticator
			if err := yaml.Unmarshal(b, &obj); err != nil {
				return nil, fmt.Errorf("failed to unmarshal OIDC authenticator object from YAML: %w", err)
			}
			confObjects[obj.GetReference()] = &obj
		default:
			return nil, fmt.Errorf("unsupported kind '%s' in the auth config file", confMeta.Kind)
		}
	}

	if conf == nil {
		return nil, fmt.Errorf("no authentication configuration object found in the auth config file")
	}

	// Build all transports.
	var transports auth.TransportSet
	for i, ts := range conf.Spec.Transports {
		switch ts.Type {
		case auth.TransportBearerToken:
			transports = append(transports, auth.BearerTokenTransport{})
		case auth.TransportBasicAuth:
			transports = append(transports, auth.BasicAuthTransport{})
		case auth.TransportCustomHTTPHeader:
			if ts.Headers == nil {
				return nil, fmt.Errorf("missing headers configuration for CustomHTTPHeader transport at index %d", i)
			}
			transports = append(transports, &auth.CustomHTTPHeaderTransport{CustomHTTPHeaderSpec: *ts.Headers})
		default:
			return nil, fmt.Errorf("unsupported transport type '%s' in the auth config file", ts.Type)
		}
	}
	if len(transports) == 0 {
		return nil, fmt.Errorf("no transports found in the authentication configuration")
	}

	// Build all authenticators.
	var authenticators auth.AuthenticatorSet
	for i, ref := range conf.Spec.Authenticators {
		if ref.Kind == "" {
			return nil, fmt.Errorf("missing kind in authenticator reference at index %d", i)
		}

		if ref.Name == "" {
			return nil, fmt.Errorf("missing name in authenticator reference at index %d", i)
		}

		obj, ok := confObjects[ref]
		if !ok {
			return nil, fmt.Errorf("authenticator %s of kind %s not found in the auth config file", ref.Name, ref.Kind)
		}

		switch typedObj := obj.(type) {
		case *auth.OIDCAuthenticator:
			authenticator, err := oidc.New(typedObj.Spec)
			if err != nil {
				return nil, fmt.Errorf("failed to create OIDC authenticator %s: %w", ref.Name, err)
			}
			authenticators = append(authenticators, authenticator)
		default:
			return nil, fmt.Errorf("unsupported authenticator kind '%s' in the auth config file", ref.Kind)
		}
	}
	if len(authenticators) == 0 {
		return nil, fmt.Errorf("no authenticators found in the authentication configuration")
	}

	return auth.NewMiddleware(transports, authenticators, conf.Spec.ValidateScopes), nil
}
