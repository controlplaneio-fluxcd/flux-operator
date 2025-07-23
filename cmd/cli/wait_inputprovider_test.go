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

func TestWaitInputProviderCmd(t *testing.T) {
	gt := NewWithT(t)
	ctxInit, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctxInit, "test-wait-inputprovider")
	gt.Expect(err).ToNot(HaveOccurred())

	// Create inputprovider
	defaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
		"env": "test",
	})
	gt.Expect(err).ToNot(HaveOccurred())

	inputprovider := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-inputprovider",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type:          fluxcdv1.InputProviderStatic,
			DefaultValues: defaultValues,
		},
	}
	gt.Expect(testClient.Create(ctxInit, inputprovider)).To(Succeed())
	defer func() {
		_ = testClient.Delete(ctxInit, inputprovider)
	}()

	tests := []struct {
		name           string
		readyCondition metav1.Condition
		expectError    bool
		expectOutput   string
	}{
		{
			name: "inputprovider with no Ready condition",
			readyCondition: metav1.Condition{
				Type:               meta.HealthyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "test",
				Message:            "test",
			},
			expectError:  true,
			expectOutput: fmt.Sprintf("waiting for %s/%s to become ready", ns.Name, inputprovider.Name),
		},
		{
			name: "inputprovider with Ready condition False",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationFailed",
				Message:            "ResourceSetInputProvider reconciliation failed",
			},
			expectError:  true,
			expectOutput: "ReconciliationFailed: ResourceSetInputProvider reconciliation failed",
		},
		{
			name: "inputprovider with Ready condition Unknown",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "Progressing",
				Message:            "ResourceSetInputProvider reconciliation is in progress",
			},
			expectError:  true,
			expectOutput: "timed out waiting",
		},
		{
			name: "inputprovider with Ready condition True",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationSucceeded",
				Message:            "ResourceSetInputProvider reconciliation succeeded",
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

			inputprovider.Status = fluxcdv1.ResourceSetInputProviderStatus{
				Conditions: []metav1.Condition{tt.readyCondition},
			}
			g.Expect(testClient.Status().Update(ctx, inputprovider)).To(Succeed())

			// Execute command
			rootArgs.timeout = 4 * time.Second
			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand([]string{"wait", "inputprovider", "test-inputprovider"})

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
