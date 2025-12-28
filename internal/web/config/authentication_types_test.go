// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAuthenticationSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *AuthenticationSpec
		wantErr string
	}{
		{
			name:    "nil spec is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name: "valid Anonymous authentication",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{
					Username: "test-user",
				},
			},
			wantErr: "",
		},
		{
			name: "valid OAuth2 authentication",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeOAuth2,
				OAuth2: &OAuth2AuthenticationSpec{
					Provider:     OAuth2ProviderOIDC,
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					IssuerURL:    "https://issuer.example.com",
				},
			},
			wantErr: "",
		},
		{
			name: "invalid authentication type",
			spec: &AuthenticationSpec{
				Type: "InvalidType",
			},
			wantErr: "invalid authentication type 'InvalidType'",
		},
		{
			name: "unconfigured authentication type",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
			},
			wantErr: "authentication type 'Anonymous' is not configured",
		},
		{
			name: "multiple authentication types configured",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{
					Username: "test-user",
				},
				OAuth2: &OAuth2AuthenticationSpec{
					Provider:     OAuth2ProviderOIDC,
					ClientID:     "client-id",
					ClientSecret: "client-secret",
					IssuerURL:    "https://issuer.example.com",
				},
			},
			wantErr: "multiple authentication configurations found",
		},
		{
			name: "invalid Anonymous configuration",
			spec: &AuthenticationSpec{
				Type:      AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{},
			},
			wantErr: "invalid Anonymous authentication configuration",
		},
		{
			name: "invalid OAuth2 configuration",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeOAuth2,
				OAuth2: &OAuth2AuthenticationSpec{
					Provider: OAuth2ProviderOIDC,
				},
			},
			wantErr: "invalid OAuth2 authentication configuration",
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

func TestAuthenticationSpec_ApplyDefaults(t *testing.T) {
	for _, tt := range []struct {
		name                    string
		spec                    *AuthenticationSpec
		expectedSessionDuration time.Duration
		expectedUserCacheSize   int
	}{
		{
			name: "nil spec does not panic",
			spec: nil,
		},
		{
			name: "sets session duration default",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{
					Username: "test-user",
				},
			},
			expectedSessionDuration: 7 * 24 * time.Hour,
			expectedUserCacheSize:   100,
		},
		{
			name: "preserves existing session duration",
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{
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
			spec: &AuthenticationSpec{
				Type: AuthenticationTypeAnonymous,
				Anonymous: &AnonymousAuthenticationSpec{
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
			tt.spec.ApplyDefaults()
			if tt.spec != nil {
				g.Expect(tt.spec.SessionDuration.Duration).To(Equal(tt.expectedSessionDuration))
				g.Expect(tt.spec.UserCacheSize).To(Equal(tt.expectedUserCacheSize))
			}
		})
	}
}

func TestAnonymousAuthenticationSpec_Configured(t *testing.T) {
	g := NewWithT(t)

	var nilSpec *AnonymousAuthenticationSpec
	g.Expect(nilSpec.Configured()).To(BeFalse())

	spec := &AnonymousAuthenticationSpec{}
	g.Expect(spec.Configured()).To(BeTrue())
}

func TestAnonymousAuthenticationSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *AnonymousAuthenticationSpec
		wantErr string
	}{
		{
			name:    "missing both username and groups",
			spec:    &AnonymousAuthenticationSpec{},
			wantErr: "at least one of 'username' or 'groups' must be set",
		},
		{
			name: "has username only",
			spec: &AnonymousAuthenticationSpec{
				Username: "test-user",
			},
			wantErr: "",
		},
		{
			name: "has groups only",
			spec: &AnonymousAuthenticationSpec{
				Groups: []string{"group1", "group2"},
			},
			wantErr: "",
		},
		{
			name: "has both username and groups",
			spec: &AnonymousAuthenticationSpec{
				Username: "test-user",
				Groups:   []string{"group1"},
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

func TestAnonymousAuthenticationSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *AnonymousAuthenticationSpec
	nilSpec.ApplyDefaults()

	// nil groups becomes empty slice
	spec := &AnonymousAuthenticationSpec{
		Username: "test-user",
	}
	spec.ApplyDefaults()
	g.Expect(spec.Groups).NotTo(BeNil())
	g.Expect(spec.Groups).To(BeEmpty())

	// existing groups are preserved
	spec2 := &AnonymousAuthenticationSpec{
		Username: "test-user",
		Groups:   []string{"group1"},
	}
	spec2.ApplyDefaults()
	g.Expect(spec2.Groups).To(Equal([]string{"group1"}))
}

func TestOAuth2AuthenticationSpec_Configured(t *testing.T) {
	g := NewWithT(t)

	var nilSpec *OAuth2AuthenticationSpec
	g.Expect(nilSpec.Configured()).To(BeFalse())

	spec := &OAuth2AuthenticationSpec{}
	g.Expect(spec.Configured()).To(BeTrue())
}

func TestOAuth2AuthenticationSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *OAuth2AuthenticationSpec
		wantErr string
	}{
		{
			name: "missing clientID",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
			},
			wantErr: "clientID must be set",
		},
		{
			name: "missing clientSecret",
			spec: &OAuth2AuthenticationSpec{
				Provider:  OAuth2ProviderOIDC,
				ClientID:  "client-id",
				IssuerURL: "https://issuer.example.com",
			},
			wantErr: "clientSecret must be set",
		},
		{
			name: "OIDC missing issuerURL",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantErr: "issuerURL must be set for the OIDC OAuth2 provider",
		},
		{
			name: "invalid provider",
			spec: &OAuth2AuthenticationSpec{
				Provider:     "InvalidProvider",
				ClientID:     "client-id",
				ClientSecret: "secret",
			},
			wantErr: "invalid OAuth2 provider: 'InvalidProvider'",
		},
		{
			name: "valid OIDC config",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
			},
			wantErr: "",
		},
		{
			name: "valid OIDC config with scopes",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				Scopes:       []string{"openid", "profile", "email"},
			},
			wantErr: "",
		},
		{
			name: "OIDC with invalid variable CEL expression",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				ClaimsProcessorSpec: ClaimsProcessorSpec{
					Variables: []VariableSpec{
						{Name: "test", Expression: "invalid[[["},
					},
				},
			},
			wantErr: "invalid variable[0]",
		},
		{
			name: "OIDC with invalid impersonation",
			spec: &OAuth2AuthenticationSpec{
				Provider:     OAuth2ProviderOIDC,
				ClientID:     "client-id",
				ClientSecret: "secret",
				IssuerURL:    "https://issuer.example.com",
				ClaimsProcessorSpec: ClaimsProcessorSpec{
					Impersonation: &ImpersonationSpec{},
				},
			},
			wantErr: "invalid impersonation",
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

func TestOAuth2AuthenticationSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *OAuth2AuthenticationSpec
	nilSpec.ApplyDefaults()

	// OIDC provider applies claims processor defaults
	spec := &OAuth2AuthenticationSpec{
		Provider:     OAuth2ProviderOIDC,
		ClientID:     "client-id",
		ClientSecret: "secret",
		IssuerURL:    "https://issuer.example.com",
	}
	spec.ApplyDefaults()
	g.Expect(spec.Profile).NotTo(BeNil())
	g.Expect(spec.Profile.Name).NotTo(BeEmpty())
	g.Expect(spec.Impersonation).NotTo(BeNil())
	g.Expect(spec.Impersonation.Username).NotTo(BeEmpty())
	g.Expect(spec.Impersonation.Groups).NotTo(BeEmpty())
}

func TestClaimsProcessorSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    ClaimsProcessorSpec
		wantErr string
	}{
		{
			name:    "empty spec is valid",
			spec:    ClaimsProcessorSpec{},
			wantErr: "",
		},
		{
			name: "valid config with variables and validations",
			spec: ClaimsProcessorSpec{
				Variables: []VariableSpec{
					{Name: "email", Expression: "claims.email"},
				},
				Validations: []ValidationSpec{
					{Expression: "variables.email != ''", Message: "Email required"},
				},
			},
			wantErr: "",
		},
		{
			name: "invalid variable",
			spec: ClaimsProcessorSpec{
				Variables: []VariableSpec{
					{Name: "", Expression: "claims.email"},
				},
			},
			wantErr: "invalid variable[0]",
		},
		{
			name: "invalid validation",
			spec: ClaimsProcessorSpec{
				Validations: []ValidationSpec{
					{Expression: "", Message: "test"},
				},
			},
			wantErr: "invalid validation[0]",
		},
		{
			name: "invalid profile",
			spec: ClaimsProcessorSpec{
				Profile: &ProfileSpec{
					Name: "invalid[[[",
				},
			},
			wantErr: "invalid profile",
		},
		{
			name: "invalid impersonation",
			spec: ClaimsProcessorSpec{
				Impersonation: &ImpersonationSpec{},
			},
			wantErr: "invalid impersonation",
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

func TestClaimsProcessorSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *ClaimsProcessorSpec
	nilSpec.ApplyDefaults()

	// sets default profile and impersonation
	spec := ClaimsProcessorSpec{}
	spec.ApplyDefaults()
	g.Expect(spec.Profile).NotTo(BeNil())
	g.Expect(spec.Impersonation).NotTo(BeNil())
}

func TestVariableSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    VariableSpec
		wantErr string
	}{
		{
			name:    "missing name",
			spec:    VariableSpec{Expression: "claims.email"},
			wantErr: "variable name must be provided",
		},
		{
			name:    "missing expression",
			spec:    VariableSpec{Name: "email"},
			wantErr: "variable expression must be provided",
		},
		{
			name:    "invalid CEL expression",
			spec:    VariableSpec{Name: "test", Expression: "invalid[[["},
			wantErr: "failed to parse variable 'test' CEL expression",
		},
		{
			name:    "valid config",
			spec:    VariableSpec{Name: "email", Expression: "claims.email"},
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

func TestValidationSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    ValidationSpec
		wantErr string
	}{
		{
			name:    "missing expression",
			spec:    ValidationSpec{Message: "test message"},
			wantErr: "validation expression must be provided",
		},
		{
			name:    "missing message",
			spec:    ValidationSpec{Expression: "true"},
			wantErr: "validation message must be provided",
		},
		{
			name:    "invalid CEL expression",
			spec:    ValidationSpec{Expression: "invalid[[[", Message: "test"},
			wantErr: "failed to parse validation CEL expression",
		},
		{
			name:    "valid config",
			spec:    ValidationSpec{Expression: "true", Message: "always pass"},
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

func TestProfileSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *ProfileSpec
		wantErr string
	}{
		{
			name:    "nil is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "empty spec is valid",
			spec:    &ProfileSpec{},
			wantErr: "",
		},
		{
			name:    "valid CEL expression",
			spec:    &ProfileSpec{Name: "claims.name"},
			wantErr: "",
		},
		{
			name:    "invalid CEL expression",
			spec:    &ProfileSpec{Name: "invalid[[["},
			wantErr: "failed to parse name expression",
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

func TestProfileSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *ProfileSpec
	nilSpec.ApplyDefaults()

	// sets default name expression
	spec := &ProfileSpec{}
	spec.ApplyDefaults()
	g.Expect(spec.Name).To(Equal("has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')"))

	// preserves existing name
	spec2 := &ProfileSpec{Name: "claims.preferred_username"}
	spec2.ApplyDefaults()
	g.Expect(spec2.Name).To(Equal("claims.preferred_username"))
}

func TestImpersonationSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *ImpersonationSpec
		wantErr string
	}{
		{
			name:    "nil is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "both empty fails",
			spec:    &ImpersonationSpec{},
			wantErr: "impersonation must have at least one of username or groups expressions",
		},
		{
			name:    "username only valid",
			spec:    &ImpersonationSpec{Username: "claims.email"},
			wantErr: "",
		},
		{
			name:    "groups only valid",
			spec:    &ImpersonationSpec{Groups: "claims.groups"},
			wantErr: "",
		},
		{
			name:    "both valid",
			spec:    &ImpersonationSpec{Username: "claims.email", Groups: "claims.groups"},
			wantErr: "",
		},
		{
			name:    "invalid username CEL",
			spec:    &ImpersonationSpec{Username: "invalid[[["},
			wantErr: "failed to parse username expression",
		},
		{
			name:    "invalid groups CEL",
			spec:    &ImpersonationSpec{Username: "claims.email", Groups: "invalid[[["},
			wantErr: "failed to parse groups expression",
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

func TestImpersonationSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *ImpersonationSpec
	nilSpec.ApplyDefaults()

	// sets default username and groups expressions
	spec := &ImpersonationSpec{}
	spec.ApplyDefaults()
	g.Expect(spec.Username).To(Equal("has(claims.email) ? claims.email : ''"))
	g.Expect(spec.Groups).To(Equal("has(claims.groups) ? claims.groups : []"))

	// preserves existing values
	spec2 := &ImpersonationSpec{
		Username: "claims.sub",
		Groups:   "claims.roles",
	}
	spec2.ApplyDefaults()
	g.Expect(spec2.Username).To(Equal("claims.sub"))
	g.Expect(spec2.Groups).To(Equal("claims.roles"))
}
