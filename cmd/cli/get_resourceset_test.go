// Copyright 2025 Stefan Prodan.
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

func TestGetResourceSetCmd(t *testing.T) {
	var (
		defaultColumns       = []string{"NAME", "RESOURCES", "READY", "MESSAGE", "LAST RECONCILED"}
		allNamespacesColumns = []string{"NAMESPACE", "NAME", "RESOURCES", "READY", "MESSAGE", "LAST RECONCILED"}
	)
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
				"ready-app", "1", "True", "ResourceSet reconciliation succeeded",
			},
		},
		{
			name:           "single namespace with failed resource",
			setupResources: true,
			expectOutput: []string{
				"failed-app", "0", "False", "ResourceSet reconciliation failed",
			},
		},
		{
			name:           "all namespaces",
			setupResources: true,
			allNamespaces:  true,
			expectOutput: []string{
				"ready-app", "1", "True", "ResourceSet reconciliation succeeded",
				"failed-app", "0", "False", "ResourceSet reconciliation failed",
				"suspended-app", "0", "Suspended",
			},
		},
		{
			name:           "specific resource by name",
			setupResources: true,
			args:           []string{"ready-app"},
			expectOutput: []string{
				"ready-app", "1", "True", "ResourceSet reconciliation succeeded",
			},
		},
		{
			name:           "suspended resource",
			setupResources: true,
			allNamespaces:  true,
			expectOutput: []string{
				"suspended-app", "0", "Suspended",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			var ns1, ns2 *corev1.Namespace

			// Create ResourceSets if needed
			if tt.setupResources {
				var err error
				ns1, err = testEnv.CreateNamespace(ctx, "test1")
				g.Expect(err).ToNot(HaveOccurred())
				ns2, err = testEnv.CreateNamespace(ctx, "test2")
				g.Expect(err).ToNot(HaveOccurred())

				// Create ready ResourceSet in ns1
				readyInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "my-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				readyResourceSet := &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ready-app",
						Namespace: ns1.Name,
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs: []fluxcdv1.ResourceSetInput{readyInput},
						ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>-config
data:
  app: "<< .inputs.app >>"`,
					},
				}
				err = testClient.Create(ctx, readyResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				// Set ready status
				readyResourceSet.Status = fluxcdv1.ResourceSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "ResourceSet reconciliation succeeded",
						},
					},
					Inventory: &fluxcdv1.ResourceInventory{
						Entries: []fluxcdv1.ResourceRef{
							{
								ID:      fmt.Sprintf("%s_my-app-config_%s_ConfigMap", ns1.Name, ""),
								Version: "v1",
							},
						},
					},
				}
				err = testClient.Status().Update(ctx, readyResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				// Create failed ResourceSet in ns1
				failedInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "broken-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				failedResourceSet := &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "failed-app",
						Namespace: ns1.Name,
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs: []fluxcdv1.ResourceSetInput{failedInput},
						ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>-config
  namespace: nonexistent-namespace
data:
  app: "<< .inputs.app >>"`,
					},
				}
				err = testClient.Create(ctx, failedResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				// Set failed status
				failedResourceSet.Status = fluxcdv1.ResourceSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationFailedReason,
							Message:            "ResourceSet reconciliation failed",
						},
					},
				}
				err = testClient.Status().Update(ctx, failedResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				// Create suspended ResourceSet in ns2
				suspendedInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "suspended-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				suspendedResourceSet := &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "suspended-app",
						Namespace: ns2.Name,
						Annotations: map[string]string{
							fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
						},
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs: []fluxcdv1.ResourceSetInput{suspendedInput},
						ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>-config
data:
  app: "<< .inputs.app >>"`,
					},
				}
				err = testClient.Create(ctx, suspendedResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				suspendedResourceSet.Status = fluxcdv1.ResourceSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "ResourceSet reconciliation succeeded",
						},
					},
				}
				err = testClient.Status().Update(ctx, suspendedResourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, readyResourceSet)
					_ = testClient.Delete(ctx, failedResourceSet)
					_ = testClient.Delete(ctx, suspendedResourceSet)
				}()

				// Set namespace for single namespace tests
				if !tt.allNamespaces {
					kubeconfigArgs.Namespace = &ns1.Name
				}
			}

			// Prepare command arguments
			args := []string{"get", "resourceset"}
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
