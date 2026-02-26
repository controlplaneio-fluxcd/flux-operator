// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestReconcileResourcesCmd(t *testing.T) {
	tests := []struct {
		name           string
		setupResources bool
		args           []string
		expectError    bool
		expectOutput   []string
	}{
		{
			name:        "missing kind flag",
			args:        []string{"reconcile", "all"},
			expectError: true,
		},
		{
			name:        "invalid kind",
			args:        []string{"reconcile", "all", "--kind=NonExistent"},
			expectError: true,
		},
		{
			name:           "reconcile all resourcesets in namespace",
			setupResources: true,
			args:           []string{"reconcile", "all", "--kind=ResourceSet"},
			expectOutput: []string{
				"triggered for",
				"ready-app",
				"failed-app",
			},
		},
		{
			name:           "reconcile all namespaces",
			setupResources: true,
			args:           []string{"reconcile", "all", "--kind=ResourceSet", "-A"},
			expectOutput: []string{
				"triggered for",
				"ready-app",
				"failed-app",
			},
		},
		{
			name:           "reconcile with ready status filter",
			setupResources: true,
			args:           []string{"reconcile", "all", "--kind=ResourceSet", "--ready-status=False"},
			expectOutput: []string{
				"triggered for",
				"failed-app",
			},
		},
		{
			name:        "no resources found",
			args:        []string{"reconcile", "all", "--kind=ResourceSet"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			if tt.setupResources {
				ns, err := testEnv.CreateNamespace(ctx, "test-reconcile-all")
				g.Expect(err).ToNot(HaveOccurred())

				readyInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "ready",
				})
				g.Expect(err).ToNot(HaveOccurred())

				readyRS := &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ready-app",
						Namespace: ns.Name,
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs: []fluxcdv1.ResourceSetInput{readyInput},
						ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>
data:
  app: "<< .inputs.app >>"`,
					},
				}
				err = testClient.Create(ctx, readyRS)
				g.Expect(err).ToNot(HaveOccurred())

				readyRS.Status = fluxcdv1.ResourceSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "Applied successfully",
						},
					},
				}
				err = testClient.Status().Update(ctx, readyRS)
				g.Expect(err).ToNot(HaveOccurred())

				failedInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "failed",
				})
				g.Expect(err).ToNot(HaveOccurred())

				failedRS := &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "failed-app",
						Namespace: ns.Name,
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs: []fluxcdv1.ResourceSetInput{failedInput},
						ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>
data:
  app: "<< .inputs.app >>"`,
					},
				}
				err = testClient.Create(ctx, failedRS)
				g.Expect(err).ToNot(HaveOccurred())

				failedRS.Status = fluxcdv1.ResourceSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionFalse,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationFailedReason,
							Message:            "Apply failed",
						},
					},
				}
				err = testClient.Status().Update(ctx, failedRS)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, readyRS)
					_ = testClient.Delete(ctx, failedRS)
				}()

				kubeconfigArgs.Namespace = &ns.Name
			}

			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			for _, expected := range tt.expectOutput {
				g.Expect(output).To(ContainSubstring(expected))
			}
		})
	}
}
