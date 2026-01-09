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
			name: "valid audit with single action",
			spec: &UserActionsSpec{
				Audit: []string{UserActionReconcile},
			},
			wantErr: "",
		},
		{
			name: "valid audit with multiple actions",
			spec: &UserActionsSpec{
				Audit: []string{UserActionReconcile, UserActionSuspend, UserActionResume},
			},
			wantErr: "",
		},
		{
			name: "valid audit with wildcard",
			spec: &UserActionsSpec{
				Audit: []string{"*"},
			},
			wantErr: "",
		},
		{
			name: "duplicate audit action",
			spec: &UserActionsSpec{
				Audit: []string{UserActionReconcile, UserActionSuspend, UserActionReconcile},
			},
			wantErr: "duplicate audit action: 'reconcile'",
		},
		{
			name: "invalid audit action",
			spec: &UserActionsSpec{
				Audit: []string{"invalid-action"},
			},
			wantErr: "invalid audit action: 'invalid-action'",
		},
		{
			name: "wildcard combined with other actions",
			spec: &UserActionsSpec{
				Audit: []string{"*", UserActionReconcile},
			},
			wantErr: "audit action '*' cannot be combined with other actions",
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
		Audit: []string{UserActionReconcile},
	}
	spec.ApplyDefaults()
	g.Expect(spec.Audit).To(Equal([]string{UserActionReconcile}))

	// empty spec applies defaults without error
	spec2 := &UserActionsSpec{}
	spec2.ApplyDefaults()
	g.Expect(spec2.Audit).To(BeNil())
}

func TestAllUserActions(t *testing.T) {
	g := NewWithT(t)

	// Verify AllUserActions contains the expected actions
	g.Expect(AllUserActions).To(ConsistOf(UserActionReconcile, UserActionSuspend, UserActionResume))
	g.Expect(AllUserActions).To(HaveLen(3))
}
