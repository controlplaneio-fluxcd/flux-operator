// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfig_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "wrong API version",
			config: Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "wrong/v1",
					Kind:       ConfigKind,
				},
			},
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1'",
		},
		{
			name: "wrong kind",
			config: Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: GroupVersion.String(),
					Kind:       "WrongKind",
				},
			},
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1' and kind 'Config'",
		},
		{
			name: "valid config with Anonymous auth",
			config: Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: GroupVersion.String(),
					Kind:       ConfigKind,
				},
				Spec: ConfigSpec{
					Authentication: &AuthenticationSpec{
						Type: AuthenticationTypeAnonymous,
						Anonymous: &AnonymousAuthenticationSpec{
							Username: "test-user",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid config with OAuth2 auth",
			config: Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: GroupVersion.String(),
					Kind:       ConfigKind,
				},
				Spec: ConfigSpec{
					BaseURL: "https://status.example.com",
					Authentication: &AuthenticationSpec{
						Type: AuthenticationTypeOAuth2,
						OAuth2: &OAuth2AuthenticationSpec{
							Provider:     OAuth2ProviderOIDC,
							ClientID:     "client-id",
							ClientSecret: "secret",
							IssuerURL:    "https://issuer.example.com",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid config without auth",
			config: Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: GroupVersion.String(),
					Kind:       ConfigKind,
				},
				Spec: ConfigSpec{},
			},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := tt.config.Validate()
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestConfigSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    ConfigSpec
		wantErr string
	}{
		{
			name:    "empty spec is valid",
			spec:    ConfigSpec{},
			wantErr: "",
		},
		{
			name: "OAuth2 without baseURL fails",
			spec: ConfigSpec{
				Authentication: &AuthenticationSpec{
					Type: AuthenticationTypeOAuth2,
					OAuth2: &OAuth2AuthenticationSpec{
						Provider:     OAuth2ProviderOIDC,
						ClientID:     "client-id",
						ClientSecret: "secret",
						IssuerURL:    "https://issuer.example.com",
					},
				},
			},
			wantErr: "baseURL must be set when OAuth2 authentication is configured",
		},
		{
			name: "valid baseURL",
			spec: ConfigSpec{
				BaseURL: "https://status.example.com",
			},
			wantErr: "",
		},
		{
			name: "baseURL with path",
			spec: ConfigSpec{
				BaseURL: "https://status.example.com/flux",
			},
			wantErr: "",
		},
		{
			name: "auth validation errors propagate",
			spec: ConfigSpec{
				Authentication: &AuthenticationSpec{
					Type: "InvalidType",
				},
			},
			wantErr: "invalid authentication configuration",
		},
		{
			name: "insecure mode is allowed",
			spec: ConfigSpec{
				Insecure: true,
			},
			wantErr: "",
		},
		{
			name: "Anonymous auth does not require baseURL",
			spec: ConfigSpec{
				Authentication: &AuthenticationSpec{
					Type: AuthenticationTypeAnonymous,
					Anonymous: &AnonymousAuthenticationSpec{
						Username: "test-user",
					},
				},
			},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := tt.spec.Validate()
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestConfigSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// spec with auth applies auth defaults
	spec := &ConfigSpec{
		Authentication: &AuthenticationSpec{
			Type: AuthenticationTypeAnonymous,
			Anonymous: &AnonymousAuthenticationSpec{
				Username: "test-user",
			},
		},
	}
	spec.ApplyDefaults()
	g.Expect(spec.Authentication.SessionDuration).NotTo(BeNil())
	g.Expect(spec.Authentication.UserCacheSize).To(Equal(100))

	// spec without auth does not panic
	spec2 := &ConfigSpec{}
	spec2.ApplyDefaults()
}
