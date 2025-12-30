// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestUserActionsSpec_Validate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *UserActionsSpec
		wantErr string
	}{
		{
			name:    "nil spec is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "empty spec is valid",
			spec:    &UserActionsSpec{},
			wantErr: "",
		},
		{
			name: "valid reconcile action",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionReconcile},
			},
			wantErr: "",
		},
		{
			name: "valid suspend action",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionSuspend},
			},
			wantErr: "",
		},
		{
			name: "valid resume action",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionResume},
			},
			wantErr: "",
		},
		{
			name: "valid multiple actions",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionReconcile, UserActionSuspend, UserActionResume},
			},
			wantErr: "",
		},
		{
			name: "valid with audit enabled",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionReconcile},
				Audit:   true,
			},
			wantErr: "",
		},
		{
			name: "unknown action",
			spec: &UserActionsSpec{
				Enabled: []string{"unknown"},
			},
			wantErr: "unknown user action: 'unknown'",
		},
		{
			name: "unknown action among valid ones",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionReconcile, "invalid", UserActionSuspend},
			},
			wantErr: "unknown user action: 'invalid'",
		},
		{
			name: "duplicate action",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionReconcile, UserActionSuspend, UserActionReconcile},
			},
			wantErr: "duplicate user action: 'reconcile'",
		},
		{
			name: "duplicate action at beginning",
			spec: &UserActionsSpec{
				Enabled: []string{UserActionSuspend, UserActionSuspend},
			},
			wantErr: "duplicate user action: 'suspend'",
		},
		{
			name: "valid authType Anonymous",
			spec: &UserActionsSpec{
				AuthType: AuthenticationTypeAnonymous,
			},
			wantErr: "",
		},
		{
			name: "valid authType OAuth2",
			spec: &UserActionsSpec{
				AuthType: AuthenticationTypeOAuth2,
			},
			wantErr: "",
		},
		{
			name: "invalid authType",
			spec: &UserActionsSpec{
				AuthType: "InvalidType",
			},
			wantErr: "invalid authType: 'InvalidType'",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var err error
			if tt.spec != nil {
				err = tt.spec.Validate()
			}
			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestUserActionsSpec_ApplyDefaults(t *testing.T) {
	g := NewWithT(t)

	// nil spec does not panic
	var nilSpec *UserActionsSpec
	nilSpec.ApplyDefaults()

	// spec with values preserves them
	spec := &UserActionsSpec{
		Enabled:  []string{UserActionReconcile},
		AuthType: AuthenticationTypeAnonymous,
		Audit:    true,
	}
	spec.ApplyDefaults()
	g.Expect(spec.Enabled).To(Equal([]string{UserActionReconcile}))
	g.Expect(spec.AuthType).To(Equal(AuthenticationTypeAnonymous))
	g.Expect(spec.Audit).To(BeTrue())

	// empty authType defaults to OAuth2
	spec2 := &UserActionsSpec{}
	spec2.ApplyDefaults()
	g.Expect(spec2.AuthType).To(Equal(AuthenticationTypeOAuth2))
}

func TestAllUserActions(t *testing.T) {
	g := NewWithT(t)

	// Verify AllUserActions contains the expected actions
	g.Expect(AllUserActions).To(ConsistOf(UserActionReconcile, UserActionSuspend, UserActionResume))
	g.Expect(AllUserActions).To(HaveLen(3))
}

func TestUserActionsSpec_IsEnabled(t *testing.T) {
	for _, tt := range []struct {
		name     string
		spec     *UserActionsSpec
		action   string
		expected bool
	}{
		{
			name:     "nil spec enables all actions",
			spec:     nil,
			action:   UserActionReconcile,
			expected: true,
		},
		{
			name:     "nil Enabled enables all actions",
			spec:     &UserActionsSpec{Enabled: nil},
			action:   UserActionReconcile,
			expected: true,
		},
		{
			name:     "empty Enabled disables all actions",
			spec:     &UserActionsSpec{Enabled: []string{}},
			action:   UserActionReconcile,
			expected: false,
		},
		{
			name:     "action in Enabled list is enabled",
			spec:     &UserActionsSpec{Enabled: []string{UserActionReconcile, UserActionSuspend}},
			action:   UserActionReconcile,
			expected: true,
		},
		{
			name:     "action not in Enabled list is disabled",
			spec:     &UserActionsSpec{Enabled: []string{UserActionReconcile, UserActionSuspend}},
			action:   UserActionResume,
			expected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.spec.IsEnabled(tt.action)).To(Equal(tt.expected))
		})
	}
}
