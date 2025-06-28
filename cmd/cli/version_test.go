// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestVersionCmd(t *testing.T) {
	tests := []struct {
		name            string
		clientOnly      bool
		setupReport     bool
		operatorVer     string
		distributionVer string
		expectedOutput  []string
		expectError     bool
	}{
		{
			name:       "client only",
			clientOnly: true,
			expectedOutput: []string{
				"client: " + VERSION,
			},
		},
		{
			name:        "no server",
			expectError: true,
		},
		{
			name:            "with server",
			setupReport:     true,
			operatorVer:     "v1.2.3",
			distributionVer: "v2.4.0",
			expectedOutput: []string{
				"client: " + VERSION,
				"server: v1.2.3",
				"distribution: v2.4.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Create FluxReport if needed
			if tt.setupReport {
				ns, err := testEnv.CreateNamespace(ctx, "test")
				g.Expect(err).ToNot(HaveOccurred())

				report := &fluxcdv1.FluxReport{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flux",
						Namespace: ns.Name,
					},
					Spec: fluxcdv1.FluxReportSpec{
						Distribution: fluxcdv1.FluxDistributionStatus{
							Entitlement: "oss",
							Status:      "ready",
						},
					},
				}

				if tt.operatorVer != "" {
					report.Spec.Operator = &fluxcdv1.OperatorInfo{
						APIVersion: "v1",
						Version:    tt.operatorVer,
						Platform:   "linux/amd64",
					}
				}

				if tt.distributionVer != "" {
					report.Spec.Distribution.Version = tt.distributionVer
				}

				err = testClient.Create(ctx, report)
				g.Expect(err).ToNot(HaveOccurred())
				defer func() {
					_ = testClient.Delete(ctx, report)
				}()
			}

			// Prepare command arguments
			args := []string{"version"}
			if tt.clientOnly {
				args = append(args, "--client")
			}

			// Execute command
			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Check output
			for _, expected := range tt.expectedOutput {
				g.Expect(output).To(ContainSubstring(expected))
			}
		})
	}
}
