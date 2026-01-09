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
	// Audit is a list of actions to be audited.
	// If the field is empty or omitted, no actions are audited.
	// The special value ["*"] can be used to audit all actions.
	// +optional
	Audit []string `json:"audit,omitempty"`
}

// Validate validates the UserActionsSpec configuration.
func (u *UserActionsSpec) Validate() error {
	auditedActions := make(map[string]struct{})
	for _, action := range u.Audit {
		if _, exists := auditedActions[action]; exists {
			return fmt.Errorf("duplicate audit action: '%s'", action)
		}
		if !slices.Contains(AllUserActions, action) && action != "*" {
			return fmt.Errorf("invalid audit action: '%s'", action)
		}
		auditedActions[action] = struct{}{}
	}
	if _, exists := auditedActions["*"]; exists && len(auditedActions) > 1 {
		return fmt.Errorf("audit action '*' cannot be combined with other actions")
	}
	return nil
}

// ApplyDefaults applies default values to the UserActionsSpec.
func (u *UserActionsSpec) ApplyDefaults() {
	if u == nil {
		return
	}
}
