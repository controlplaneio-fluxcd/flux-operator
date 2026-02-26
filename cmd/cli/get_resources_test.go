// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestGetResourcesCmd(t *testing.T) {
	tests := []struct {
		name           string
		setupResources bool
		args           []string
		expectError    bool
		expectOutput   []string
		noExpectOutput []string
	}{
		{
			name:        "no resources found",
			args:        []string{"get", "all"},
			expectError: true,
		},
		{
			name:           "list all namespaces",
			setupResources: true,
			args:           []string{"get", "all", "-A"},
			expectOutput: []string{
				"ResourceSet", "ready-app", "True", "Applied successfully",
				"ResourceSet", "failed-app", "False", "Apply failed",
				"Suspended",
			},
		},
		{
			name:           "list single namespace",
			setupResources: true,
			args:           []string{"get", "all"},
			expectOutput: []string{
				"ResourceSet", "ready-app", "True", "Applied successfully",
				"ResourceSet", "failed-app", "False", "Apply failed",
			},
			noExpectOutput: []string{
				"suspended-app",
			},
		},
		{
			name:           "filter by kind",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--kind=ResourceSet"},
			expectOutput: []string{
				"ResourceSet", "ready-app",
			},
		},
		{
			name:           "filter by kind short name",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--kind=rset"},
			expectOutput: []string{
				"ResourceSet", "ready-app",
			},
		},
		{
			name:           "filter by ready status True",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--ready-status=True"},
			expectOutput: []string{
				"ready-app",
			},
			noExpectOutput: []string{
				"failed-app",
			},
		},
		{
			name:           "filter by ready status Suspended",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--ready-status=Suspended"},
			expectOutput: []string{
				"suspended-app",
			},
			noExpectOutput: []string{
				"ready-app",
				"failed-app",
			},
		},
		{
			name:        "invalid kind",
			args:        []string{"get", "all", "--kind=NonExistent"},
			expectError: true,
		},
		{
			name:           "output json",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--kind=ResourceSet", "-o", "json"},
			expectOutput: []string{
				"ready-app",
			},
		},
		{
			name:           "output yaml",
			setupResources: true,
			args:           []string{"get", "all", "-A", "--kind=ResourceSet", "-o", "yaml"},
			expectOutput: []string{
				"ready-app",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			var ns1, ns2 *corev1.Namespace

			if tt.setupResources {
				var err error
				ns1, err = testEnv.CreateNamespace(ctx, "test1")
				g.Expect(err).ToNot(HaveOccurred())
				ns2, err = testEnv.CreateNamespace(ctx, "test2")
				g.Expect(err).ToNot(HaveOccurred())

				readyInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "my-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				readyRS := &fluxcdv1.ResourceSet{
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
					"app": "broken-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				failedRS := &fluxcdv1.ResourceSet{
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

				suspendedInput, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "suspended-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				suspendedRS := &fluxcdv1.ResourceSet{
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
				err = testClient.Create(ctx, suspendedRS)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, readyRS)
					_ = testClient.Delete(ctx, failedRS)
					_ = testClient.Delete(ctx, suspendedRS)
				}()

				// Set namespace for single namespace tests.
				kubeconfigArgs.Namespace = &ns1.Name
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

			for _, notExpected := range tt.noExpectOutput {
				g.Expect(output).ToNot(ContainSubstring(notExpected))
			}
		})
	}
}

func TestGetResourcesCmd_JSONOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	g := NewWithT(t)

	ns, err := testEnv.CreateNamespace(ctx, "test-json")
	g.Expect(err).ToNot(HaveOccurred())

	input, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
		"app": "json-app",
	})
	g.Expect(err).ToNot(HaveOccurred())

	rs := &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "json-app",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			Inputs: []fluxcdv1.ResourceSetInput{input},
			ResourcesTemplate: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: << .inputs.app >>
data:
  app: "<< .inputs.app >>"`,
		},
	}
	err = testClient.Create(ctx, rs)
	g.Expect(err).ToNot(HaveOccurred())

	rs.Status = fluxcdv1.ResourceSetStatus{
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
	err = testClient.Status().Update(ctx, rs)
	g.Expect(err).ToNot(HaveOccurred())
	defer func() {
		_ = testClient.Delete(ctx, rs)
	}()

	kubeconfigArgs.Namespace = &ns.Name

	// Verify JSON output is valid.
	output, err := executeCommand([]string{"get", "all", "-o", "json"})
	g.Expect(err).ToNot(HaveOccurred())

	var result []ResourceStatus
	err = json.Unmarshal([]byte(output), &result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).ToNot(BeEmpty())
	g.Expect(result[0].Kind).To(Equal("ResourceSet"))
	g.Expect(result[0].Ready).To(Equal("True"))

	// Verify YAML output is valid.
	output, err = executeCommand([]string{"get", "all", "-o", "yaml"})
	g.Expect(err).ToNot(HaveOccurred())

	var yamlResult []ResourceStatus
	err = yaml.Unmarshal([]byte(output), &yamlResult)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(yamlResult).ToNot(BeEmpty())
	g.Expect(yamlResult[0].Kind).To(Equal("ResourceSet"))
	g.Expect(yamlResult[0].Ready).To(Equal("True"))
}
