// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateWebConfig(t *testing.T) {
	for _, tt := range []struct {
		name    string
		config  fluxcdv1.WebConfig
		wantErr string
	}{
		{
			name: "wrong API version",
			config: fluxcdv1.WebConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "wrong/v1",
					Kind:       fluxcdv1.WebConfigKind,
				},
			},
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1'",
		},
		{
			name: "wrong kind",
			config: fluxcdv1.WebConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.WebConfigGroupVersion.String(),
					Kind:       "WrongKind",
				},
			},
			wantErr: "expected apiVersion 'web.fluxcd.controlplane.io/v1' and kind 'Config'",
		},
		{
			name: "valid config with Anonymous auth",
			config: fluxcdv1.WebConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.WebConfigGroupVersion.String(),
					Kind:       fluxcdv1.WebConfigKind,
				},
				Spec: fluxcdv1.WebConfigSpec{
					Authentication: &fluxcdv1.AuthenticationSpec{
						Type: fluxcdv1.AuthenticationTypeAnonymous,
						Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
							Username: "test-user",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid config with OAuth2 auth",
			config: fluxcdv1.WebConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.WebConfigGroupVersion.String(),
					Kind:       fluxcdv1.WebConfigKind,
				},
				Spec: fluxcdv1.WebConfigSpec{
					BaseURL: "https://status.example.com",
					Authentication: &fluxcdv1.AuthenticationSpec{
						Type: fluxcdv1.AuthenticationTypeOAuth2,
						OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
							Provider:     fluxcdv1.OAuth2ProviderOIDC,
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
			config: fluxcdv1.WebConfig{
				TypeMeta: metav1.TypeMeta{
					APIVersion: fluxcdv1.WebConfigGroupVersion.String(),
					Kind:       fluxcdv1.WebConfigKind,
				},
				Spec: fluxcdv1.WebConfigSpec{},
			},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateWebConfig(&tt.config)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestValidateWebConfigSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    fluxcdv1.WebConfigSpec
		wantErr string
	}{
		{
			name:    "empty spec is valid",
			spec:    fluxcdv1.WebConfigSpec{},
			wantErr: "",
		},
		{
			name: "OAuth2 without baseURL fails",
			spec: fluxcdv1.WebConfigSpec{
				Authentication: &fluxcdv1.AuthenticationSpec{
					Type: fluxcdv1.AuthenticationTypeOAuth2,
					OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
						Provider:     fluxcdv1.OAuth2ProviderOIDC,
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
			spec: fluxcdv1.WebConfigSpec{
				BaseURL: "https://status.example.com",
			},
			wantErr: "",
		},
		{
			name: "baseURL with path",
			spec: fluxcdv1.WebConfigSpec{
				BaseURL: "https://status.example.com/flux",
			},
			wantErr: "",
		},
		{
			name: "auth validation errors propagate",
			spec: fluxcdv1.WebConfigSpec{
				Authentication: &fluxcdv1.AuthenticationSpec{
					Type: "InvalidType",
				},
			},
			wantErr: "invalid authentication configuration",
		},
		{
			name: "valid userActions config",
			spec: fluxcdv1.WebConfigSpec{
				UserActions: &fluxcdv1.UserActionsSpec{
					Audit: []string{fluxcdv1.UserActionReconcile},
				},
			},
			wantErr: "",
		},
		{
			name: "insecure mode is allowed",
			spec: fluxcdv1.WebConfigSpec{
				Insecure: true,
			},
			wantErr: "",
		},
		{
			name: "Anonymous auth does not require baseURL",
			spec: fluxcdv1.WebConfigSpec{
				Authentication: &fluxcdv1.AuthenticationSpec{
					Type: fluxcdv1.AuthenticationTypeAnonymous,
					Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
						Username: "test-user",
					},
				},
			},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateWebConfigSpec(&tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyWebConfigSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// spec with auth applies auth defaults
	spec := &fluxcdv1.WebConfigSpec{
		Authentication: &fluxcdv1.AuthenticationSpec{
			Type: fluxcdv1.AuthenticationTypeAnonymous,
			Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
			},
		},
	}
	ApplyWebConfigSpecDefaults(spec)
	g.Expect(spec.Authentication.SessionDuration).NotTo(BeNil())
	g.Expect(spec.Authentication.UserCacheSize).To(Equal(100))
	// UserActions should be initialized
	g.Expect(spec.UserActions).NotTo(BeNil())

	// spec without auth does not panic and initializes UserActions
	spec2 := &fluxcdv1.WebConfigSpec{}
	ApplyWebConfigSpecDefaults(spec2)
	g.Expect(spec2.UserActions).NotTo(BeNil())

	// spec with userActions preserves values
	spec3 := &fluxcdv1.WebConfigSpec{
		UserActions: &fluxcdv1.UserActionsSpec{
			Audit: []string{fluxcdv1.UserActionReconcile},
		},
	}
	ApplyWebConfigSpecDefaults(spec3)
	g.Expect(spec3.UserActions.Audit).To(Equal([]string{fluxcdv1.UserActionReconcile}))
}

func TestWebConfigSpec_UserActionsEnabled(t *testing.T) {
	for _, tt := range []struct {
		name     string
		spec     *fluxcdv1.WebConfigSpec
		expected bool
	}{
		{
			name:     "nil spec disables actions",
			spec:     nil,
			expected: false,
		},
		{
			name:     "nil authentication disables actions",
			spec:     &fluxcdv1.WebConfigSpec{},
			expected: false,
		},
		{
			name: "OAuth2 auth enables actions",
			spec: &fluxcdv1.WebConfigSpec{
				Authentication: &fluxcdv1.AuthenticationSpec{Type: fluxcdv1.AuthenticationTypeOAuth2},
			},
			expected: true,
		},
		{
			name: "Anonymous auth enables actions",
			spec: &fluxcdv1.WebConfigSpec{
				Authentication: &fluxcdv1.AuthenticationSpec{Type: fluxcdv1.AuthenticationTypeAnonymous},
			},
			expected: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.spec.UserActionsEnabled()).To(Equal(tt.expected))
		})
	}
}
