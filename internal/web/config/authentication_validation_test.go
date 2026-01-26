// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateAuthenticationSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *fluxcdv1.AuthenticationSpec
		wantErr string
	}{
		{
			name: "valid Anonymous authentication",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
					Username: "test-user",
				},
			},
			wantErr: "",
		},
		{
			name: "valid OAuth2 authentication",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
				OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
					Provider:     fluxcdv1.OAuth2ProviderOIDC,
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					IssuerURL:    "https://issuer.example.com",
				},
			},
			wantErr: "",
		},
		{
			name: "invalid authentication type",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: "InvalidType",
			},
			wantErr: "invalid authentication type 'InvalidType'",
		},
		{
			name: "unconfigured authentication type",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
			},
			wantErr: "authentication type 'Anonymous' is not configured",
		},
		{
			name: "multiple authentication types configured",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
					Username: "test-user",
				},
				OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
					Provider:     fluxcdv1.OAuth2ProviderOIDC,
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					IssuerURL:    "https://issuer.example.com",
				},
			},
			wantErr: "multiple authentication configurations found",
		},
		{
			name: "invalid Anonymous configuration",
			spec: &fluxcdv1.AuthenticationSpec{
				Type:      fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{},
			},
			wantErr: "invalid Anonymous authentication configuration",
		},
		{
			name: "invalid OAuth2 configuration",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeOAuth2,
				OAuth2: &fluxcdv1.OAuth2AuthenticationSpec{
					Provider: fluxcdv1.OAuth2ProviderOIDC,
				},
			},
			wantErr: "invalid OAuth2 authentication configuration",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateAuthenticationSpec(tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyAuthenticationSpecDefaults(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		spec                    *fluxcdv1.AuthenticationSpec
		expectedSessionDuration time.Duration
		expectedUserCacheSize   int
	}{
		{
			name: "nil spec does not panic",
			spec: nil,
		},
		{
			name: "sets session duration default",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
					Username: "test-user",
				},
			},
			expectedSessionDuration: 7 * 24 * time.Hour,
			expectedUserCacheSize:   100,
		},
		{
			name: "preserves existing session duration",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
					Username: "test-user",
				},
				SessionDuration: &metav1.Duration{Duration: 2 * time.Hour},
				UserCacheSize:   50,
			},
			expectedSessionDuration: 2 * time.Hour,
			expectedUserCacheSize:   50,
		},
		{
			name: "replaces zero session duration with default",
			spec: &fluxcdv1.AuthenticationSpec{
				Type: fluxcdv1.AuthenticationTypeAnonymous,
				Anonymous: &fluxcdv1.AnonymousAuthenticationSpec{
					Username: "test-user",
				},
				SessionDuration: &metav1.Duration{Duration: 0},
			},
			expectedSessionDuration: 7 * 24 * time.Hour,
			expectedUserCacheSize:   100,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ApplyAuthenticationSpecDefaults(tt.spec)
			if tt.spec != nil {
				g.Expect(tt.spec.SessionDuration.Duration).To(Equal(tt.expectedSessionDuration))
				g.Expect(tt.spec.UserCacheSize).To(Equal(tt.expectedUserCacheSize))
			}
		})
	}
}

func TestAnonymousAuthenticationSpec_Configured(t *testing.T) {
	g := NewWithT(t)

	var nilSpec *fluxcdv1.AnonymousAuthenticationSpec
	g.Expect(nilSpec.Configured()).To(BeFalse())

	spec := &fluxcdv1.AnonymousAuthenticationSpec{}
	g.Expect(spec.Configured()).To(BeTrue())
}

func TestValidateAnonymousAuthenticationSpec(t *testing.T) {
	for _, tt := range []struct {
		name         string
		spec         *fluxcdv1.AnonymousAuthenticationSpec
		wantErr      string
		wantUsername string
		wantGroups   []string
	}{
		{
			name:    "missing both username and groups",
			spec:    &fluxcdv1.AnonymousAuthenticationSpec{},
			wantErr: "at least one of 'username' or 'groups' must be set",
		},
		{
			name: "has username only",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
			},
			wantUsername: "test-user",
			wantGroups:   nil,
		},
		{
			name: "has groups only",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Groups: []string{"group1", "group2"},
			},
			wantUsername: "",
			wantGroups:   []string{"group1", "group2"},
		},
		{
			name: "has both username and groups",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
				Groups:   []string{"group1"},
			},
			wantUsername: "test-user",
			wantGroups:   []string{"group1"},
		},
		{
			name: "trims whitespace from username",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "  test-user  ",
			},
			wantUsername: "test-user",
			wantGroups:   nil,
		},
		{
			name: "trims whitespace from groups",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Groups: []string{"  group1  ", "  group2  "},
			},
			wantUsername: "",
			wantGroups:   []string{"group1", "group2"},
		},
		{
			name: "sorts groups alphabetically",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
				Groups:   []string{"zebra", "alpha", "middle"},
			},
			wantUsername: "test-user",
			wantGroups:   []string{"alpha", "middle", "zebra"},
		},
		{
			name: "whitespace-only username with no groups fails",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "   ",
			},
			wantErr: "at least one of 'username' or 'groups' must be set",
		},
		{
			name: "empty string in groups fails",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
				Groups:   []string{"group1", ""},
			},
			wantErr: "group[0] is an empty string",
		},
		{
			name: "whitespace-only group fails",
			spec: &fluxcdv1.AnonymousAuthenticationSpec{
				Username: "test-user",
				Groups:   []string{"group1", "   "},
			},
			wantErr: "group[0] is an empty string",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateAnonymousAuthenticationSpec(tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(tt.spec.Username).To(Equal(tt.wantUsername))
				g.Expect(tt.spec.Groups).To(Equal(tt.wantGroups))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyAnonymousAuthenticationSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *fluxcdv1.AnonymousAuthenticationSpec
	ApplyAnonymousAuthenticationSpecDefaults(nilSpec)

	// nil groups becomes empty slice
	spec := &fluxcdv1.AnonymousAuthenticationSpec{
		Username: "test-user",
	}
	ApplyAnonymousAuthenticationSpecDefaults(spec)
	g.Expect(spec.Groups).NotTo(BeNil())
	g.Expect(spec.Groups).To(BeEmpty())

	// existing groups are preserved
	spec2 := &fluxcdv1.AnonymousAuthenticationSpec{
		Username: "test-user",
		Groups:   []string{"group1"},
	}
	ApplyAnonymousAuthenticationSpecDefaults(spec2)
	g.Expect(spec2.Groups).To(Equal([]string{"group1"}))
}

func TestOAuth2AuthenticationSpec_Configured(t *testing.T) {
	g := NewWithT(t)

	var nilSpec *fluxcdv1.OAuth2AuthenticationSpec
	g.Expect(nilSpec.Configured()).To(BeFalse())

	spec := &fluxcdv1.OAuth2AuthenticationSpec{}
	g.Expect(spec.Configured()).To(BeTrue())
}

func TestValidateOAuth2AuthenticationSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *fluxcdv1.OAuth2AuthenticationSpec
		wantErr string
	}{
		{
			name: "missing clientID",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
			},
			wantErr: "clientID must be set",
		},
		{
			name: "missing clientSecret",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:  fluxcdv1.OAuth2ProviderOIDC,
				ClientID:  "client-id",
				IssuerURL: "https://issuer.example.com",
			},
			wantErr: "clientSecret must be set",
		},
		{
			name: "OIDC missing issuerURL",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantErr: "issuerURL must be set for the OIDC OAuth2 provider",
		},
		{
			name: "invalid provider",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     "InvalidProvider",
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantErr: "invalid OAuth2 provider: 'InvalidProvider'",
		},
		{
			name: "valid OIDC config",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
			},
			wantErr: "",
		},
		{
			name: "valid OIDC config with scopes",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				Scopes:       []string{"openid", "profile", "email"},
			},
			wantErr: "",
		},
		{
			name: "OIDC with invalid variable CEL expression",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				ClaimsProcessorSpec: fluxcdv1.ClaimsProcessorSpec{
					Variables: []fluxcdv1.VariableSpec{
						{Name: "test", Expression: "invalid[[["},
					},
				},
			},
			wantErr: "invalid variable[0]",
		},
		{
			name: "OIDC with invalid impersonation",
			spec: &fluxcdv1.OAuth2AuthenticationSpec{
				Provider:     fluxcdv1.OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				ClaimsProcessorSpec: fluxcdv1.ClaimsProcessorSpec{
					Impersonation: &fluxcdv1.ImpersonationSpec{},
				},
			},
			wantErr: "invalid impersonation",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateOAuth2AuthenticationSpec(tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyOAuth2AuthenticationSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *fluxcdv1.OAuth2AuthenticationSpec
	ApplyOAuth2AuthenticationSpecDefaults(nilSpec)

	// OIDC provider applies claims processor defaults
	spec := &fluxcdv1.OAuth2AuthenticationSpec{
		Provider:     fluxcdv1.OAuth2ProviderOIDC,
		ClientID:     "client-id",
		ClientSecret: "secret",
		IssuerURL:    "https://issuer.example.com",
	}
	ApplyOAuth2AuthenticationSpecDefaults(spec)
	g.Expect(spec.Profile).NotTo(BeNil())
	g.Expect(spec.Profile.Name).NotTo(BeEmpty())
	g.Expect(spec.Impersonation).NotTo(BeNil())
	g.Expect(spec.Impersonation.Username).NotTo(BeEmpty())
	g.Expect(spec.Impersonation.Groups).NotTo(BeEmpty())
}

func TestValidateClaimsProcessorSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    fluxcdv1.ClaimsProcessorSpec
		wantErr string
	}{
		{
			name:    "empty spec is valid",
			spec:    fluxcdv1.ClaimsProcessorSpec{},
			wantErr: "",
		},
		{
			name: "valid config with variables and validations",
			spec: fluxcdv1.ClaimsProcessorSpec{
				Variables: []fluxcdv1.VariableSpec{
					{Name: "email", Expression: "claims.email"},
				},
				Validations: []fluxcdv1.ValidationSpec{
					{Expression: "variables.email != ''", Message: "Email required"},
				},
			},
			wantErr: "",
		},
		{
			name: "invalid variable",
			spec: fluxcdv1.ClaimsProcessorSpec{
				Variables: []fluxcdv1.VariableSpec{
					{Name: "", Expression: "claims.email"},
				},
			},
			wantErr: "invalid variable[0]",
		},
		{
			name: "invalid validation",
			spec: fluxcdv1.ClaimsProcessorSpec{
				Validations: []fluxcdv1.ValidationSpec{
					{Expression: "", Message: "test"},
				},
			},
			wantErr: "invalid validation[0]",
		},
		{
			name: "invalid profile",
			spec: fluxcdv1.ClaimsProcessorSpec{
				Profile: &fluxcdv1.ProfileSpec{
					Name: "invalid[[[",
				},
			},
			wantErr: "invalid profile",
		},
		{
			name: "invalid impersonation",
			spec: fluxcdv1.ClaimsProcessorSpec{
				Impersonation: &fluxcdv1.ImpersonationSpec{},
			},
			wantErr: "invalid impersonation",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateClaimsProcessorSpec(&tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyClaimsProcessorSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *fluxcdv1.ClaimsProcessorSpec
	ApplyClaimsProcessorSpecDefaults(nilSpec)

	// sets default profile and impersonation
	spec := fluxcdv1.ClaimsProcessorSpec{}
	ApplyClaimsProcessorSpecDefaults(&spec)
	g.Expect(spec.Profile).NotTo(BeNil())
	g.Expect(spec.Impersonation).NotTo(BeNil())
}

func TestValidateVariableSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    fluxcdv1.VariableSpec
		wantErr string
	}{
		{
			name:    "missing name",
			spec:    fluxcdv1.VariableSpec{Expression: "claims.email"},
			wantErr: "variable name must be provided",
		},
		{
			name:    "missing expression",
			spec:    fluxcdv1.VariableSpec{Name: "email"},
			wantErr: "variable expression must be provided",
		},
		{
			name:    "invalid CEL expression",
			spec:    fluxcdv1.VariableSpec{Name: "test", Expression: "invalid[[["},
			wantErr: "failed to parse variable 'test' CEL expression",
		},
		{
			name:    "valid config",
			spec:    fluxcdv1.VariableSpec{Name: "email", Expression: "claims.email"},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateVariableSpec(&tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestValidateValidationSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    fluxcdv1.ValidationSpec
		wantErr string
	}{
		{
			name:    "missing expression",
			spec:    fluxcdv1.ValidationSpec{Message: "test message"},
			wantErr: "validation expression must be provided",
		},
		{
			name:    "missing message",
			spec:    fluxcdv1.ValidationSpec{Expression: "true"},
			wantErr: "validation message must be provided",
		},
		{
			name:    "invalid CEL expression",
			spec:    fluxcdv1.ValidationSpec{Expression: "invalid[[[", Message: "test"},
			wantErr: "failed to parse validation CEL expression",
		},
		{
			name:    "valid config",
			spec:    fluxcdv1.ValidationSpec{Expression: "true", Message: "always pass"},
			wantErr: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateValidationSpec(&tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestValidateProfileSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *fluxcdv1.ProfileSpec
		wantErr string
	}{
		{
			name:    "nil is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "empty spec is valid",
			spec:    &fluxcdv1.ProfileSpec{},
			wantErr: "",
		},
		{
			name:    "valid CEL expression",
			spec:    &fluxcdv1.ProfileSpec{Name: "claims.name"},
			wantErr: "",
		},
		{
			name:    "invalid CEL expression",
			spec:    &fluxcdv1.ProfileSpec{Name: "invalid[[["},
			wantErr: "failed to parse name expression",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateProfileSpec(tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyProfileSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *fluxcdv1.ProfileSpec
	ApplyProfileSpecDefaults(nilSpec)

	// sets default name expression
	spec := &fluxcdv1.ProfileSpec{}
	ApplyProfileSpecDefaults(spec)
	g.Expect(spec.Name).To(Equal("has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')"))

	// preserves existing name
	spec2 := &fluxcdv1.ProfileSpec{Name: "claims.preferred_username"}
	ApplyProfileSpecDefaults(spec2)
	g.Expect(spec2.Name).To(Equal("claims.preferred_username"))
}

func TestValidateImpersonationSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *fluxcdv1.ImpersonationSpec
		wantErr string
	}{
		{
			name:    "nil is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "both empty fails",
			spec:    &fluxcdv1.ImpersonationSpec{},
			wantErr: "impersonation must have at least one of username or groups expressions",
		},
		{
			name:    "username only valid",
			spec:    &fluxcdv1.ImpersonationSpec{Username: "claims.email"},
			wantErr: "",
		},
		{
			name:    "groups only valid",
			spec:    &fluxcdv1.ImpersonationSpec{Groups: "claims.groups"},
			wantErr: "",
		},
		{
			name:    "both valid",
			spec:    &fluxcdv1.ImpersonationSpec{Username: "claims.email", Groups: "claims.groups"},
			wantErr: "",
		},
		{
			name:    "invalid username CEL",
			spec:    &fluxcdv1.ImpersonationSpec{Username: "invalid[[["},
			wantErr: "failed to parse username expression",
		},
		{
			name:    "invalid groups CEL",
			spec:    &fluxcdv1.ImpersonationSpec{Username: "claims.email", Groups: "invalid[[["},
			wantErr: "failed to parse groups expression",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := ValidateImpersonationSpec(tt.spec)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestApplyImpersonationSpecDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *fluxcdv1.ImpersonationSpec
	ApplyImpersonationSpecDefaults(nilSpec)

	// sets default username and groups expressions
	spec := &fluxcdv1.ImpersonationSpec{}
	ApplyImpersonationSpecDefaults(spec)
	g.Expect(spec.Username).To(Equal("has(claims.email) ? claims.email : ''"))
	g.Expect(spec.Groups).To(Equal("has(claims.groups) ? claims.groups : []"))

	// preserves existing values
	spec2 := &fluxcdv1.ImpersonationSpec{
		Username: "claims.sub",
		Groups:   "claims.roles",
	}
	ApplyImpersonationSpecDefaults(spec2)
	g.Expect(spec2.Username).To(Equal("claims.sub"))
	g.Expect(spec2.Groups).To(Equal("claims.roles"))
}

func TestAllAuthenticationTypes(t *testing.T) {
	g := NewWithT(t)

	// Verify AllAuthenticationTypes contains the expected types
	g.Expect(fluxcdv1.AllAuthenticationTypes).To(ConsistOf(fluxcdv1.AuthenticationTypeAnonymous, fluxcdv1.AuthenticationTypeOAuth2))
	g.Expect(fluxcdv1.AllAuthenticationTypes).To(HaveLen(2))
}
