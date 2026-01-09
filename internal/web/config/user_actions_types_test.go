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
			name: "valid with audit enabled",
			spec: &UserActionsSpec{
				Audit: true,
			},
			wantErr: "",
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
		AuthType: AuthenticationTypeAnonymous,
		Audit:    true,
	}
	spec.ApplyDefaults()
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
