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
	// Enabled lists which user actions are enabled. If not set, all actions are considered enabled.
	// To disable all user actions, set this field to an empty list.
	// Note that actions are only available when authentication is configured with the OAuth2 type.
	// +optional
	Enabled []string `json:"enabled,omitempty"`

	// Audit indicates whether to send audit events when users perform actions.
	// +optional
	Audit bool `json:"audit,omitempty"`
}

// Validate validates the UserActionsSpec configuration.
func (u *UserActionsSpec) Validate() error {
	actions := make(map[string]struct{})
	for _, action := range u.Enabled {
		if _, exists := actions[action]; exists {
			return fmt.Errorf("duplicate user action: '%s'", action)
		}
		if !slices.Contains(AllUserActions, action) {
			return fmt.Errorf("unknown user action: '%s'", action)
		}
		actions[action] = struct{}{}
	}
	return nil
}

// ApplyDefaults applies default values to the UserActionsSpec.
func (u *UserActionsSpec) ApplyDefaults() {
	if u == nil {
		return
	}
}
