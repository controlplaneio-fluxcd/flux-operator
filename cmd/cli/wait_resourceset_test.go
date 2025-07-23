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

func TestWaitResourceSetCmd(t *testing.T) {
	gt := NewWithT(t)
	ctxInit, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctxInit, "test-wait-resourceset")
	gt.Expect(err).ToNot(HaveOccurred())

	// Create resourceset
	input, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
		"app": "test-app",
	})
	gt.Expect(err).ToNot(HaveOccurred())

	resourceset := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resourceset",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Inputs: []fluxcdv1.ResourceSetInput{input},
			ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>-config
data:
  app: "<< .inputs.app >>"`,
		},
	}
	gt.Expect(testClient.Create(ctxInit, resourceset)).To(Succeed())
	defer func() {
		_ = testClient.Delete(ctxInit, resourceset)
	}()

	tests := []struct {
		name           string
		readyCondition metav1.Condition
		expectError    bool
		expectOutput   string
	}{
		{
			name: "resourceset with no ObservedGeneration mismatch",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 2,
				LastTransitionTime: metav1.Now(),
				Reason:             "test",
				Message:            "test",
			},
			expectError:  true,
			expectOutput: fmt.Sprintf("waiting for %s/%s to become ready", ns.Name, resourceset.Name),
		},
		{
			name: "resourceset with Ready condition False",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionFalse,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationFailed",
				Message:            "ResourceSet reconciliation failed",
			},
			expectError:  true,
			expectOutput: "ReconciliationFailed: ResourceSet reconciliation failed",
		},
		{
			name: "resourceset with Ready condition Unknown",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "Progressing",
				Message:            "ResourceSet reconciliation is in progress",
			},
			expectError:  true,
			expectOutput: "timed out waiting",
		},
		{
			name: "resourceset with Ready condition True",
			readyCondition: metav1.Condition{
				Type:               meta.ReadyCondition,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
				LastTransitionTime: metav1.Now(),
				Reason:             "ReconciliationSucceeded",
				Message:            "ResourceSet reconciliation succeeded",
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

			resourceset.Status = fluxcdv1.ResourceSetStatus{
				Conditions: []metav1.Condition{tt.readyCondition},
			}
			g.Expect(testClient.Status().Update(ctx, resourceset)).To(Succeed())

			// Execute command
			rootArgs.timeout = 4 * time.Second
			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand([]string{"wait", "resourceset", "test-resourceset"})

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
