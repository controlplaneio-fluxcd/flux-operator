// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestExportReportCmd(t *testing.T) {
	tests := []struct {
		name        string
		setupReport bool
		expectError bool
	}{
		{
			name:        "no report",
			expectError: true,
		},
		{
			name:        "with report",
			setupReport: true,
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
						Labels: map[string]string{
							"app.kubernetes.io/name": "flux-operator",
							"fluxcd.io/name":         "flux",
							"fluxcd.io/namespace":    ns.Name,
						},
						Annotations: map[string]string{
							meta.ReconcileRequestAnnotation: "2024-01-01T00:00:00Z",
						},
					},
					Spec: fluxcdv1.FluxReportSpec{
						Distribution: fluxcdv1.FluxDistributionStatus{
							Entitlement: "oss",
							Status:      "ready",
							Version:     "v1.2.3",
						},
						Operator: &fluxcdv1.OperatorInfo{
							APIVersion: "v1",
							Version:    "v1.2.3",
							Platform:   "linux/amd64",
						},
					},
				}

				err = testClient.Create(ctx, report)
				g.Expect(err).ToNot(HaveOccurred())

				// Set status to ready
				report.Status = fluxcdv1.FluxReportStatus{
					Conditions: []metav1.Condition{
						{
							Type:               meta.ReadyCondition,
							Status:             metav1.ConditionTrue,
							ObservedGeneration: 1,
							LastTransitionTime: metav1.Now(),
							Reason:             meta.ReconciliationSucceededReason,
							Message:            "report is ready",
						},
					},
				}
				err = testClient.Status().Update(ctx, report)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, report)
				}()
			}

			// Execute command
			output, err := executeCommand([]string{"export", "report"})

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Parse output as unstructured object
			obj := &unstructured.Unstructured{}
			err = yaml.Unmarshal([]byte(output), &obj.Object)
			g.Expect(err).ToNot(HaveOccurred())

			// Verify basic structure
			g.Expect(obj.GetAPIVersion()).To(Equal("fluxcd.controlplane.io/v1"))
			g.Expect(obj.GetKind()).To(Equal("FluxReport"))
			g.Expect(obj.GetName()).To(Equal("flux"))
			g.Expect(obj.GetNamespace()).ToNot(BeEmpty())

			// Verify spec content using unstructured.Get
			entitlement, found, err := unstructured.NestedString(obj.Object, "spec", "distribution", "entitlement")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(entitlement).To(Equal("oss"))

			status, found, err := unstructured.NestedString(obj.Object, "spec", "distribution", "status")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(status).To(Equal("ready"))

			version, found, err := unstructured.NestedString(obj.Object, "spec", "distribution", "version")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(version).To(Equal("v1.2.3"))

			operatorVersion, found, err := unstructured.NestedString(obj.Object, "spec", "operator", "version")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(operatorVersion).To(Equal("v1.2.3"))

			// Verify clean export - status and unwanted metadata should be removed
			_, found, err = unstructured.NestedSlice(obj.Object, "status", "conditions")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeFalse())

			_, found, err = unstructured.NestedString(obj.Object, "metadata", "resourceVersion")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeFalse())

			_, found, err = unstructured.NestedString(obj.Object, "metadata", "uid")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeFalse())

			_, found, err = unstructured.NestedString(obj.Object, "metadata", "creationTimestamp")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeFalse())

			// Verify Flux annotations and labels are removed
			_, found, err = unstructured.NestedStringMap(obj.Object, "metadata", "annotations")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).ToNot(BeTrue())

			labels, found, err := unstructured.NestedStringMap(obj.Object, "metadata", "labels")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(labels).ToNot(HaveKey("fluxcd.io/name"))
			g.Expect(labels).ToNot(HaveKey("fluxcd.io/namespace"))
		})
	}
}

func TestExportReportCmdOutputFormat(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat string
		expectError  bool
	}{
		{
			name:         "yaml output",
			outputFormat: "yaml",
		},
		{
			name:         "json output",
			outputFormat: "json",
		},
		{
			name:         "invalid output format",
			outputFormat: "invalid",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Create FluxReport
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
						Version:     "v1.2.3",
					},
				},
			}

			err = testClient.Create(ctx, report)
			g.Expect(err).ToNot(HaveOccurred())

			defer func() {
				_ = testClient.Delete(ctx, report)
			}()

			// Execute command with output format
			output, err := executeCommand([]string{"export", "report", "-o", tt.outputFormat})

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Parse output based on format
			obj := &unstructured.Unstructured{}
			switch tt.outputFormat {
			case "json":
				g.Expect(output).To(HavePrefix("{"))
				err = json.Unmarshal([]byte(output), &obj.Object)
			case "yaml":
				g.Expect(output).NotTo(HavePrefix("{"))
				err = yaml.Unmarshal([]byte(output), &obj.Object)
			}
			g.Expect(err).ToNot(HaveOccurred())

			// Verify basic structure
			g.Expect(obj.GetAPIVersion()).To(Equal("fluxcd.controlplane.io/v1"))
			g.Expect(obj.GetKind()).To(Equal("FluxReport"))
			g.Expect(obj.GetName()).To(Equal("flux"))

			// Verify spec content
			entitlement, found, err := unstructured.NestedString(obj.Object, "spec", "distribution", "entitlement")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(entitlement).To(Equal("oss"))
		})
	}
}
