// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"fmt"
	"slices"
)

const (
	// UserActionReconcile is the reconcile user action.
	UserActionReconcile = "reconcile"

	// UserActionSuspend is the suspend user action.
	UserActionSuspend = "suspend"

	// UserActionResume is the resume user action.
	UserActionResume = "resume"
)

var (
	// AllUserActions lists all possible user actions.
	AllUserActions = []string{
		UserActionReconcile,
		UserActionSuspend,
		UserActionResume,
	}
)

// UserActionsSpec holds the actions configuration.
type UserActionsSpec struct {
	// AuthType specifies the authentication type required for enabling user actions.
	// Defaults to OAuth2.
	// +kubebuilder:validation:Enum=Anonymous;OAuth2
	// +optional
	AuthType string `json:"authType,omitempty"`

	// Audit indicates whether to send audit events when users perform actions.
	// Defaults to false.
	// +optional
	Audit bool `json:"audit,omitempty"`
}

// Validate validates the UserActionsSpec configuration.
func (u *UserActionsSpec) Validate() error {
	if u.AuthType != "" && !slices.Contains(AllAuthenticationTypes, u.AuthType) {
		return fmt.Errorf("invalid authType: '%s'", u.AuthType)
	}
	return nil
}

// ApplyDefaults applies default values to the UserActionsSpec.
func (u *UserActionsSpec) ApplyDefaults() {
	if u == nil {
		return
	}

	if u.AuthType == "" {
		u.AuthType = AuthenticationTypeOAuth2
	}
}
