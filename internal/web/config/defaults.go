// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ApplyWebConfigSpecDefaults applies default values to the WebConfigSpec.
func ApplyWebConfigSpecDefaults(c *fluxcdv1.WebConfigSpec) {
	if c == nil {
		return
	}

	if c.UserActions == nil {
		c.UserActions = &fluxcdv1.UserActionsSpec{}
	}

	if c.Authentication != nil {
		ApplyAuthenticationSpecDefaults(c.Authentication)
	}
}

// ApplyAuthenticationSpecDefaults applies default values to the AuthenticationSpec.
func ApplyAuthenticationSpecDefaults(a *fluxcdv1.AuthenticationSpec) {
	if a == nil {
		return
	}

	ApplyAnonymousAuthenticationSpecDefaults(a.Anonymous)
	ApplyOAuth2AuthenticationSpecDefaults(a.OAuth2)

	if a.SessionDuration == nil || a.SessionDuration.Duration <= 0 {
		a.SessionDuration = &metav1.Duration{Duration: 7 * 24 * time.Hour}
	}

	if a.UserCacheSize <= 0 {
		a.UserCacheSize = 100
	}
}

// ApplyAnonymousAuthenticationSpecDefaults applies default values to the AnonymousAuthenticationSpec.
func ApplyAnonymousAuthenticationSpecDefaults(a *fluxcdv1.AnonymousAuthenticationSpec) {
	if a == nil {
		return
	}
	if a.Groups == nil {
		a.Groups = []string{}
	}
}

// ApplyOAuth2AuthenticationSpecDefaults applies default values to the OAuth2AuthenticationSpec.
func ApplyOAuth2AuthenticationSpecDefaults(o *fluxcdv1.OAuth2AuthenticationSpec) {
	if o == nil {
		return
	}

	switch o.Provider {
	case fluxcdv1.OAuth2ProviderOIDC:
		ApplyClaimsProcessorSpecDefaults(&o.ClaimsProcessorSpec)
	}
}

// ApplyClaimsProcessorSpecDefaults applies default values to the ClaimsProcessorSpec.
func ApplyClaimsProcessorSpecDefaults(c *fluxcdv1.ClaimsProcessorSpec) {
	if c == nil {
		return
	}
	if c.Profile == nil {
		c.Profile = &fluxcdv1.ProfileSpec{}
	}
	ApplyProfileSpecDefaults(c.Profile)
	if c.Impersonation == nil {
		c.Impersonation = &fluxcdv1.ImpersonationSpec{}
	}
	ApplyImpersonationSpecDefaults(c.Impersonation)
}

// ApplyProfileSpecDefaults applies default values to the ProfileSpec.
func ApplyProfileSpecDefaults(u *fluxcdv1.ProfileSpec) {
	if u == nil {
		return
	}
	if u.Name == "" {
		u.Name = "has(claims.name) ? claims.name : (has(claims.email) ? claims.email : '')"
	}
}

// ApplyImpersonationSpecDefaults applies default values to the ImpersonationSpec.
func ApplyImpersonationSpecDefaults(i *fluxcdv1.ImpersonationSpec) {
	if i == nil {
		return
	}
	if i.Username == "" {
		i.Username = "has(claims.email) ? claims.email : ''"
	}
	if i.Groups == "" {
		i.Groups = "has(claims.groups) ? claims.groups : []"
	}
}
