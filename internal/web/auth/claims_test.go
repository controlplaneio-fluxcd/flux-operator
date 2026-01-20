// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package auth

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/web/config"
)

func TestNewClaimsProcessor(t *testing.T) {
	for _, tt := range []struct {
		name    string
		conf    *config.ConfigSpec
		wantErr string
	}{
		{
			name:    "valid config creates processor",
			conf:    validOAuth2ConfigSpec(),
			wantErr: "",
		},
		{
			name: "invalid variable CEL expression returns error",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Variables = []config.VariableSpec{
					{Name: "test", Expression: "invalid[[["},
				}
				return c
			}(),
			wantErr: "Syntax error",
		},
		{
			name: "invalid validation CEL expression returns error",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Validations = []config.ValidationSpec{
					{Expression: "invalid[[[", Message: "test"},
				}
				return c
			}(),
			wantErr: "Syntax error",
		},
		{
			name: "invalid profile CEL expression returns error",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Profile = &config.ProfileSpec{
					Name: "invalid[[[",
				}
				return c
			}(),
			wantErr: "Syntax error",
		},
		{
			name: "invalid impersonation username CEL expression returns error",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "invalid[[[",
					Groups:   "[]",
				}
				return c
			}(),
			wantErr: "Syntax error",
		},
		{
			name: "invalid impersonation groups CEL expression returns error",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "claims.email",
					Groups:   "invalid[[[",
				}
				return c
			}(),
			wantErr: "Syntax error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			processor, err := newClaimsProcessor(tt.conf)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(processor).NotTo(BeNil())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestClaimsProcessorFunc(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name            string
		conf            *config.ConfigSpec
		claims          map[string]any
		wantErr         string
		wantProfileName string
		wantUsername    string
		wantGroups      []string
	}{
		{
			name: "extracts profile name from claims",
			conf: validOAuth2ConfigSpec(),
			claims: map[string]any{
				"email":  "user@example.com",
				"name":   "Test User",
				"groups": []any{"group1", "group2"},
			},
			wantProfileName: "Test User",
			wantUsername:    "user@example.com",
			wantGroups:      []string{"group1", "group2"},
		},
		{
			name: "falls back to email for profile name when name missing",
			conf: validOAuth2ConfigSpec(),
			claims: map[string]any{
				"email":  "user@example.com",
				"groups": []any{"group1"},
			},
			wantProfileName: "user@example.com",
			wantUsername:    "user@example.com",
			wantGroups:      []string{"group1"},
		},
		{
			name: "returns nil groups when groups claim missing",
			conf: validOAuth2ConfigSpec(),
			claims: map[string]any{
				"email": "user@example.com",
				"name":  "Test User",
			},
			wantProfileName: "Test User",
			wantUsername:    "user@example.com",
			wantGroups:      nil,
		},
		{
			name: "extracts variables and uses in validation",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Variables = []config.VariableSpec{
					{Name: "domain", Expression: "claims.email.split('@')[1]"},
				}
				c.Authentication.OAuth2.Validations = []config.ValidationSpec{
					{Expression: "variables.domain == 'example.com'", Message: "Only example.com allowed"},
				}
				return c
			}(),
			claims: map[string]any{
				"email":  "user@example.com",
				"name":   "Test User",
				"groups": []any{},
			},
			wantProfileName: "Test User",
			wantUsername:    "user@example.com",
			wantGroups:      nil,
		},
		{
			name: "validation fails with message when expression returns false",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Validations = []config.ValidationSpec{
					{Expression: "has(claims.admin) && claims.admin == true", Message: "Admin access required"},
				}
				return c
			}(),
			claims: map[string]any{
				"email": "user@example.com",
				"name":  "Test User",
			},
			wantErr: "validation failed: Admin access required",
		},
		{
			name: "custom impersonation expressions work",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "claims.sub",
					Groups:   "claims.roles",
				}
				return c
			}(),
			claims: map[string]any{
				"sub":   "user-123",
				"name":  "Test User",
				"roles": []any{"admin", "developer"},
			},
			wantProfileName: "Test User",
			wantUsername:    "user-123",
			wantGroups:      []string{"admin", "developer"},
		},
		{
			name: "custom profile name expression works",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Profile = &config.ProfileSpec{
					Name: "claims.preferred_username",
				}
				return c
			}(),
			claims: map[string]any{
				"email":              "user@example.com",
				"preferred_username": "cooluser",
				"groups":             []any{},
			},
			wantProfileName: "cooluser",
			wantUsername:    "user@example.com",
			wantGroups:      nil,
		},
		{
			name: "impersonation validation fails when username and groups are empty",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "''",
					Groups:   "[]",
				}
				return c
			}(),
			claims: map[string]any{
				"email": "user@example.com",
				"name":  "Test User",
			},
			wantErr: "impersonation validation failed: at least one of 'username' or 'groups' must be set",
		},
		{
			name: "impersonation validation fails when group is empty string",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "claims.email",
					Groups:   "['group1', '']",
				}
				return c
			}(),
			claims: map[string]any{
				"email": "user@example.com",
				"name":  "Test User",
			},
			wantErr: "impersonation validation failed: group[0] is an empty string",
		},
		{
			name: "impersonation sanitizes whitespace from username",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "'  user@example.com  '",
					Groups:   "[]",
				}
				return c
			}(),
			claims: map[string]any{
				"name": "Test User",
			},
			wantProfileName: "Test User",
			wantUsername:    "user@example.com",
			wantGroups:      nil,
		},
		{
			name: "impersonation sanitizes and sorts groups",
			conf: func() *config.ConfigSpec {
				c := validOAuth2ConfigSpec()
				c.Authentication.OAuth2.Impersonation = &config.ImpersonationSpec{
					Username: "claims.email",
					Groups:   "['  zebra  ', '  alpha  ']",
				}
				return c
			}(),
			claims: map[string]any{
				"email": "user@example.com",
				"name":  "Test User",
			},
			wantProfileName: "Test User",
			wantUsername:    "user@example.com",
			wantGroups:      []string{"alpha", "zebra"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			processor, err := newClaimsProcessor(tt.conf)
			g.Expect(err).NotTo(HaveOccurred())

			details, err := processor(ctx, tt.claims)
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(details).NotTo(BeNil())
				g.Expect(details.Profile.Name).To(Equal(tt.wantProfileName))
				g.Expect(details.Impersonation.Username).To(Equal(tt.wantUsername))
				g.Expect(details.Impersonation.Groups).To(Equal(tt.wantGroups))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

// validOAuth2ConfigSpec returns a valid ConfigSpec with OAuth2 authentication
// configured with default CEL expressions for testing.
func validOAuth2ConfigSpec() *config.ConfigSpec {
	conf := &config.ConfigSpec{
		BaseURL: "https://status.example.com",
		Authentication: &config.AuthenticationSpec{
			Type:            config.AuthenticationTypeOAuth2,
			SessionDuration: &metav1.Duration{Duration: 24 * time.Hour},
			UserCacheSize:   100,
			OAuth2: &config.OAuth2AuthenticationSpec{
				Provider:     config.OAuth2ProviderOIDC,
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				IssuerURL:    "https://issuer.example.com",
			},
		},
	}
	conf.Authentication.OAuth2.ApplyDefaults()
	return conf
}
