// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"testing"

	. "github.com/onsi/gomega"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestValidateUserActionsSpec(t *testing.T) {
	for _, tt := range []struct {
		name    string
		spec    *fluxcdv1.UserActionsSpec
		wantErr string
	}{
		{
			name:    "nil spec is valid",
			spec:    nil,
			wantErr: "",
		},
		{
			name:    "empty spec is valid",
			spec:    &fluxcdv1.UserActionsSpec{},
			wantErr: "",
		},
		{
			name: "valid audit with single action",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{fluxcdv1.UserActionReconcile},
			},
			wantErr: "",
		},
		{
			name: "valid audit with multiple actions",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{fluxcdv1.UserActionReconcile, fluxcdv1.UserActionSuspend, fluxcdv1.UserActionResume},
			},
			wantErr: "",
		},
		{
			name: "valid audit with wildcard",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*"},
			},
			wantErr: "",
		},
		{
			name: "duplicate audit action",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{fluxcdv1.UserActionReconcile, fluxcdv1.UserActionSuspend, fluxcdv1.UserActionReconcile},
			},
			wantErr: "duplicate audit action: 'reconcile'",
		},
		{
			name: "invalid audit action",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{"invalid-action"},
			},
			wantErr: "invalid audit action: 'invalid-action'",
		},
		{
			name: "wildcard combined with other actions",
			spec: &fluxcdv1.UserActionsSpec{
				Audit: []string{"*", fluxcdv1.UserActionReconcile},
			},
			wantErr: "audit action '*' cannot be combined with other actions",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			var err error
			if tt.spec != nil {
				err = ValidateUserActionsSpec(tt.spec)
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

func TestAllUserActions(t *testing.T) {
	g := NewWithT(t)

	// Verify AllUserActions contains the expected actions
	g.Expect(fluxcdv1.AllUserActions).To(ConsistOf(
		fluxcdv1.UserActionReconcile,
		fluxcdv1.UserActionSuspend,
		fluxcdv1.UserActionResume,
		fluxcdv1.UserActionDownload,
		fluxcdv1.UserActionRestart,
		fluxcdv1.UserActionDelete,
	))
	g.Expect(fluxcdv1.AllUserActions).To(HaveLen(6))
}
