// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestWaitInstanceCmd(t *testing.T) {
	gt := NewWithT(t)
	ctxInit, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctxInit, "test-wait-instance")
	gt.Expect(err).ToNot(HaveOccurred())

	// Create instance
	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Distribution: fluxcdv1.Distribution{
				Version:  "2.x",
				Registry: "ghcr.io/fluxcd",
			},
		},
	}
	gt.Expect(testClient.Create(ctxInit, instance)).To(Succeed())
	defer func() {
		_ = testClient.Delete(ctxInit, instance)
	}()

	tests := []struct {
		name           string
		readyCondition metav1.Condition
		expectError    bool
		expectOutput   string
	}{
		{
			name: "instance with no Ready condition",
			readyCondition: metav1.Condition{
				Type:               meta.HealthyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "test",
				Message:            "test",
			},
			expectError:  true,
			expectOutput: fmt.Sprintf("waiting for %s/%s to become ready", ns.Name, instance.Name),
		},
		{
			name: "instance with Ready condition False",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationFailed",
				Message:            "validation error",
			},
			expectError:  true,
			expectOutput: "ReconciliationFailed: validation error",
		},
		{
			name: "instance with Ready condition Unknown",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "Progressing",
				Message:            "reconciliation is in progress",
			},
			expectError:  true,
			expectOutput: "timed out waiting",
		},
		{
			name: "instance with Ready condition True",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationSucceeded",
				Message:            "Reconciliation succeeded",
			},
			expectError:  false,
			expectOutput: "succeeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			instance.Status = fluxcdv1.FluxInstanceStatus{
				Conditions: []metav1.Condition{tt.readyCondition},
			}
			g.Expect(testClient.Status().Update(ctx, instance)).To(Succeed())

			// Execute command
			rootArgs.timeout = 4 * time.Second
			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand([]string{"wait", "instance", "flux"})

			// Check expectations
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectOutput))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(output).To(ContainSubstring(tt.expectOutput))
			}
		})
	}
}
