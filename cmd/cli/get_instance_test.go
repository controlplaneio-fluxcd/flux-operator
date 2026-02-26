// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestGetInstanceCmd(t *testing.T) {
	var (
		defaultColumns       = []string{"NAME", "RESOURCES", "READY", "MESSAGE", "LAST RECONCILED"}
		allNamespacesColumns = []string{"NAMESPACE", "NAME", "RESOURCES", "READY", "MESSAGE", "LAST RECONCILED"}
	)
	tests := []struct {
		name           string
		setupResources bool
		args           []string
		singleNs       bool
		expectError    bool
		expectOutput   []string
	}{
		{
			name:         "no resources",
			expectOutput: []string{},
		},
		{
			name:           "list all namespaces",
			setupResources: true,
			expectOutput: []string{
				"flux", "1", "True", "FluxInstance reconciliation succeeded",
				"0", "False", "FluxInstance reconciliation failed",
				"Suspended",
			},
		},
		{
			name:           "list single namespace",
			setupResources: true,
			singleNs:       true,
			expectOutput: []string{
				"flux", "1", "True", "FluxInstance reconciliation succeeded",
			},
		},
		{
			name:           "suspended instance",
			setupResources: true,
			expectOutput: []string{
				"Suspended",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			var nsReady, nsFailed, nsSuspended *corev1.Namespace

			if tt.setupResources {
				var err error
				nsReady, err = testEnv.CreateNamespace(ctx, "test-ready")
				g.Expect(err).ToNot(HaveOccurred())
				nsFailed, err = testEnv.CreateNamespace(ctx, "test-failed")
				g.Expect(err).ToNot(HaveOccurred())
				nsSuspended, err = testEnv.CreateNamespace(ctx, "test-suspended")
				g.Expect(err).ToNot(HaveOccurred())

				// Create ready FluxInstance.
				readyInstance := &fluxcdv1.FluxInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flux",
						Namespace: nsReady.Name,
					},
					Spec: fluxcdv1.FluxInstanceSpec{
						Distribution: fluxcdv1.Distribution{
							Version:  "v2.x",
							Registry: "ghcr.io/fluxcd",
							Artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
						},
					},
				}
				err = testClient.Create(ctx, readyInstance)
				g.Expect(err).ToNot(HaveOccurred())

				readyInstance.Status = fluxcdv1.FluxInstanceStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "FluxInstance reconciliation succeeded",
						},
					},
					Inventory: &fluxcdv1.ResourceInventory{
						Entries: []fluxcdv1.ResourceRef{
							{
								ID:      fmt.Sprintf("%s_source-controller_%s_Deployment", nsReady.Name, "apps"),
								Version: "v1",
							},
						},
					},
				}
				err = testClient.Status().Update(ctx, readyInstance)
				g.Expect(err).ToNot(HaveOccurred())

				// Create failed FluxInstance.
				failedInstance := &fluxcdv1.FluxInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flux",
						Namespace: nsFailed.Name,
					},
					Spec: fluxcdv1.FluxInstanceSpec{
						Distribution: fluxcdv1.Distribution{
							Version:  "v2.x",
							Registry: "ghcr.io/fluxcd",
							Artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
						},
					},
				}
				err = testClient.Create(ctx, failedInstance)
				g.Expect(err).ToNot(HaveOccurred())

				failedInstance.Status = fluxcdv1.FluxInstanceStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationFailedReason,
							Message:            "FluxInstance reconciliation failed",
						},
					},
				}
				err = testClient.Status().Update(ctx, failedInstance)
				g.Expect(err).ToNot(HaveOccurred())

				// Create suspended FluxInstance.
				suspendedInstance := &fluxcdv1.FluxInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "flux",
						Namespace: nsSuspended.Name,
						Annotations: map[string]string{
							fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
						},
					},
					Spec: fluxcdv1.FluxInstanceSpec{
						Distribution: fluxcdv1.Distribution{
							Version:  "v2.x",
							Registry: "ghcr.io/fluxcd",
							Artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
						},
					},
				}
				err = testClient.Create(ctx, suspendedInstance)
				g.Expect(err).ToNot(HaveOccurred())

				suspendedInstance.Status = fluxcdv1.FluxInstanceStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "FluxInstance reconciliation succeeded",
						},
					},
				}
				err = testClient.Status().Update(ctx, suspendedInstance)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, readyInstance)
					_ = testClient.Delete(ctx, failedInstance)
					_ = testClient.Delete(ctx, suspendedInstance)
				}()

				if tt.singleNs {
					kubeconfigArgs.Namespace = &nsReady.Name
				}
			}

			// Prepare command arguments.
			args := []string{"get", "instance"}
			if tt.singleNs {
				args = append(args, "--all-namespaces=false")
			}
			if len(tt.args) > 0 {
				args = append(args, tt.args...)
			}

			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Split output into lines.
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 1 && lines[0] == "" {
				lines = []string{}
			}

			// Verify column headers.
			if len(lines) > 0 {
				headerLine := lines[0]
				expectedColumns := defaultColumns
				if !tt.singleNs {
					expectedColumns = allNamespacesColumns
				}
				for _, expectedColumn := range expectedColumns {
					g.Expect(headerLine).To(ContainSubstring(expectedColumn))
				}
			}

			// Verify expected content.
			for _, expectedContent := range tt.expectOutput {
				g.Expect(output).To(ContainSubstring(expectedContent))
			}

			// Verify table structure.
			if tt.setupResources {
				if tt.singleNs {
					// Single namespace should show header + 1 row.
					g.Expect(lines).To(HaveLen(2))
				} else {
					// All namespaces should show header + 3 rows.
					g.Expect(len(lines)).To(BeNumerically(">=", 4))

					dataLines := lines[1:]
					foundReady, foundFailed, foundSuspended := false, false, false
					for _, line := range dataLines {
						if strings.Contains(line, nsReady.Name) {
							foundReady = true
						}
						if strings.Contains(line, nsFailed.Name) {
							foundFailed = true
						}
						if strings.Contains(line, nsSuspended.Name) {
							foundSuspended = true
						}
					}
					g.Expect(foundReady).To(BeTrue())
					g.Expect(foundFailed).To(BeTrue())
					g.Expect(foundSuspended).To(BeTrue())
				}
			} else {
				g.Expect(len(lines)).To(BeNumerically("<=", 1))
			}
		})
	}
}
