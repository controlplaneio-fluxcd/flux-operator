// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestGetInputProviderCmd(t *testing.T) {
	var (
		defaultColumns       = []string{"NAME", "INPUTS", "READY", "MESSAGE", "LAST RECONCILED", "NEXT SCHEDULE"}
		allNamespacesColumns = []string{"NAMESPACE", "NAME", "INPUTS", "READY", "MESSAGE", "LAST RECONCILED", "NEXT SCHEDULE"}
	)

	fixedTransitionTime := metav1.Now()
	expectedTime := fixedTransitionTime.Format("2006-01-02 15:04:05 -0700 MST")
	scheduleTime := metav1.NewTime(time.Now().Add(time.Hour))
	expectedScheduleTime := scheduleTime.Format("2006-01-02 15:04:05 -0700 MST")

	tests := []struct {
		name           string
		setupResources bool
		args           []string
		allNamespaces  bool
		expectError    bool
		expectOutput   []string
	}{
		{
			name:         "no resources",
			expectOutput: []string{},
		},
		{
			name:           "single namespace with ready resource",
			setupResources: true,
			expectOutput: []string{
				"ready-provider", "1", "True", "ResourceSetInputProvider reconciliation succeeded",
				expectedTime,
			},
		},
		{
			name:           "single namespace with failed resource",
			setupResources: true,
			expectOutput: []string{
				"failed-provider", "0", "False", "ResourceSetInputProvider reconciliation failed",
				expectedTime,
			},
		},
		{
			name:           "all namespaces",
			setupResources: true,
			allNamespaces:  true,
			expectOutput: []string{
				"ready-provider", "1", "True", "ResourceSetInputProvider reconciliation succeeded",
				"failed-provider", "0", "False", "ResourceSetInputProvider reconciliation failed",
				"suspended-provider", "0", "Suspended",
			},
		},
		{
			name:           "specific resource by name",
			setupResources: true,
			args:           []string{"ready-provider"},
			expectOutput: []string{
				"ready-provider", "1", "True", "ResourceSetInputProvider reconciliation succeeded",
				expectedTime,
			},
		},
		{
			name:           "suspended resource",
			setupResources: true,
			allNamespaces:  true,
			expectOutput: []string{
				"suspended-provider", "0", "Suspended",
			},
		},
		{
			name:           "resource with next schedule",
			setupResources: true,
			expectOutput: []string{
				"scheduled-provider", "0", "True", "ResourceSetInputProvider reconciliation succeeded",
				expectedTime, expectedScheduleTime,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			var ns1, ns2 *corev1.Namespace

			// Create ResourceSetInputProviders if needed
			if tt.setupResources {
				var err error
				ns1, err = testEnv.CreateNamespace(ctx, "test1")
				g.Expect(err).ToNot(HaveOccurred())
				ns2, err = testEnv.CreateNamespace(ctx, "test2")
				g.Expect(err).ToNot(HaveOccurred())

				// Create ready ResourceSetInputProvider in ns1
				readyDefaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"env": "staging",
					"app": "my-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				readyProvider := &fluxcdv1.ResourceSetInputProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ready-provider",
						Namespace: ns1.Name,
					},
					Spec: fluxcdv1.ResourceSetInputProviderSpec{
						Type:          fluxcdv1.InputProviderStatic,
						DefaultValues: readyDefaultValues,
					},
				}
				err = testClient.Create(ctx, readyProvider)
				g.Expect(err).ToNot(HaveOccurred())

				// Create exported input for ready provider
				readyExportedInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"id":  readyProvider.UID,
					"env": "staging",
					"app": "my-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				// Set ready status with exported inputs
				readyProvider.Status = fluxcdv1.ResourceSetInputProviderStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: fixedTransitionTime,
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "ResourceSetInputProvider reconciliation succeeded",
						},
					},
					ExportedInputs: []fluxcdv1.ResourceSetInput{
						readyExportedInput,
					},
					LastExportedRevision: "sha256:abc123",
				}
				err = testClient.Status().Update(ctx, readyProvider)
				g.Expect(err).ToNot(HaveOccurred())

				// Create failed ResourceSetInputProvider in ns1
				failedDefaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"env": "production",
				})
				g.Expect(err).ToNot(HaveOccurred())

				failedProvider := &fluxcdv1.ResourceSetInputProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "failed-provider",
						Namespace: ns1.Name,
					},
					Spec: fluxcdv1.ResourceSetInputProviderSpec{
						Type:          fluxcdv1.InputProviderStatic,
						DefaultValues: failedDefaultValues,
					},
				}
				err = testClient.Create(ctx, failedProvider)
				g.Expect(err).ToNot(HaveOccurred())

				// Set failed status
				failedProvider.Status = fluxcdv1.ResourceSetInputProviderStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
							LastTransitionTime: fixedTransitionTime,
							Reason:             meta.ReconciliationFailedReason,
							Message:            "ResourceSetInputProvider reconciliation failed",
						},
					},
				}
				err = testClient.Status().Update(ctx, failedProvider)
				g.Expect(err).ToNot(HaveOccurred())

				// Create suspended ResourceSetInputProvider in ns2
				suspendedDefaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"env": "development",
				})
				g.Expect(err).ToNot(HaveOccurred())

				suspendedProvider := &fluxcdv1.ResourceSetInputProvider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "suspended-provider",
						Namespace: ns2.Name,
						Annotations: map[string]string{
							fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
						},
					},
					Spec: fluxcdv1.ResourceSetInputProviderSpec{
						Type:          fluxcdv1.InputProviderStatic,
						DefaultValues: suspendedDefaultValues,
					},
				}
				err = testClient.Create(ctx, suspendedProvider)
				g.Expect(err).ToNot(HaveOccurred())

				suspendedProvider.Status = fluxcdv1.ResourceSetInputProviderStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: fixedTransitionTime,
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "ResourceSetInputProvider reconciliation succeeded",
						},
					},
				}
				err = testClient.Status().Update(ctx, suspendedProvider)
				g.Expect(err).ToNot(HaveOccurred())

				// Create scheduled ResourceSetInputProvider in ns1 for specific test case
				var scheduledProvider *fluxcdv1.ResourceSetInputProvider
				if tt.name == "resource with next schedule" {
					scheduledDefaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
						"env": "scheduled",
					})
					g.Expect(err).ToNot(HaveOccurred())

					scheduledProvider = &fluxcdv1.ResourceSetInputProvider{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "scheduled-provider",
							Namespace: ns1.Name,
						},
						Spec: fluxcdv1.ResourceSetInputProviderSpec{
							Type:          fluxcdv1.InputProviderStatic,
							DefaultValues: scheduledDefaultValues,
							Schedule: []fluxcdv1.Schedule{
								{
									Cron:     "0 10 * * *",
									TimeZone: "UTC",
								},
							},
						},
					}
					err = testClient.Create(ctx, scheduledProvider)
					g.Expect(err).ToNot(HaveOccurred())

					scheduledProvider.Status = fluxcdv1.ResourceSetInputProviderStatus{
						Conditions: []metav1.Condition{
							{
								Type:               meta.ReadyCondition,
								Status:             metav1.ConditionTrue,
								ObservedGeneration: 1,
								LastTransitionTime: fixedTransitionTime,
								Reason:             meta.ReconciliationSucceededReason,
								Message:            "ResourceSetInputProvider reconciliation succeeded",
							},
						},
						NextSchedule: &fluxcdv1.NextSchedule{
							Schedule: fluxcdv1.Schedule{
								Cron:     "0 10 * * *",
								TimeZone: "UTC",
							},
							When: scheduleTime,
						},
					}
					err = testClient.Status().Update(ctx, scheduledProvider)
					g.Expect(err).ToNot(HaveOccurred())
				}

				defer func() {
					_ = testClient.Delete(ctx, readyProvider)
					_ = testClient.Delete(ctx, failedProvider)
					_ = testClient.Delete(ctx, suspendedProvider)
					if scheduledProvider != nil {
						_ = testClient.Delete(ctx, scheduledProvider)
					}
				}()

				// Set namespace for single namespace tests
				if !tt.allNamespaces {
					kubeconfigArgs.Namespace = &ns1.Name
				}
			}

			// Prepare command arguments
			args := []string{"get", "inputprovider"}
			if tt.allNamespaces {
				args = append(args, "--all-namespaces")
			}
			if len(tt.args) > 0 {
				args = append(args, tt.args...)
			}

			// Execute command
			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Split output into lines
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 1 && lines[0] == "" {
				lines = []string{}
			}

			// Verify column headers
			if len(lines) > 0 {
				headerLine := lines[0]
				expectedColumns := defaultColumns
				if tt.allNamespaces {
					expectedColumns = allNamespacesColumns
				}
				for _, expectedColumn := range expectedColumns {
					g.Expect(headerLine).To(ContainSubstring(expectedColumn))
				}
			}

			// Verify expected content
			for _, expectedContent := range tt.expectOutput {
				g.Expect(output).To(ContainSubstring(expectedContent))
			}

			// Verify table structure
			if tt.setupResources {
				// Should have header + data rows
				if tt.allNamespaces {
					// Should show resources from both namespaces
					g.Expect(len(lines)).To(BeNumerically(">=", 3)) // header + 2+ rows

					// Verify namespace columns are present
					dataLines := lines[1:]
					foundNs1, foundNs2 := false, false
					for _, line := range dataLines {
						if strings.Contains(line, ns1.Name) {
							foundNs1 = true
						}
						if strings.Contains(line, ns2.Name) {
							foundNs2 = true
						}
					}
					g.Expect(foundNs1).To(BeTrue())
					g.Expect(foundNs2).To(BeTrue())
				} else {
					// Single namespace should show only resources from that namespace
					if len(tt.args) == 0 {
						// List all in namespace - should have header + 2 rows (ready + failed)
						g.Expect(len(lines)).To(BeNumerically(">=", 2))
					} else {
						// Specific resource - should have header + 1 row
						g.Expect(lines).To(HaveLen(2))
					}
				}
			} else {
				// No resources should show only header or empty
				g.Expect(len(lines)).To(BeNumerically("<=", 1))
			}
		})
	}
}
