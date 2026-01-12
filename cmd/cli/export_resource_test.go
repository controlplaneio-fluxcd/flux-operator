// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestExportResourceCmd(t *testing.T) {
	tests := []struct {
		name          string
		setupResource bool
		resourceArg   string
		expectError   bool
	}{
		{
			name:        "no resource",
			resourceArg: "ResourceSet/nonexistent",
			expectError: true,
		},
		{
			name:          "with ResourceSet",
			setupResource: true,
			resourceArg:   "ResourceSet/config-template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			// Create ResourceSet if needed
			if tt.setupResource {
				ns, err := testEnv.CreateNamespace(ctx, "test")
				g.Expect(err).ToNot(HaveOccurred())

				objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: config-template
  namespace: "%[1]s"
  labels:
    app.kubernetes.io/name: flux-operator
    fluxcd.io/name: config-template
    fluxcd.io/namespace: "%[1]s"
  annotations:
    %[2]s: "2024-01-01T00:00:00Z"
spec:
  commonMetadata:
    annotations:
      owner: "%[1]s"
  inputs:
    - appName: my-app
      environment: staging
  resourcesTemplate: |
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: << .inputs.appName >>-config
      namespace: "%[1]s"
    data:
      app.name: "<< .inputs.appName >>"
      app.environment: "<< .inputs.environment >>"
      config.yaml: |
        app:
          name: << .inputs.appName >>
          environment: << .inputs.environment >>
          debug: false
`, ns.Name, meta.ReconcileRequestAnnotation)

				obj := &fluxcdv1.ResourceSet{}
				err = yaml.Unmarshal([]byte(objDef), obj)
				g.Expect(err).ToNot(HaveOccurred())

				err = testClient.Create(ctx, obj)
				g.Expect(err).ToNot(HaveOccurred())

				// Set status to ready
				obj.Status = fluxcdv1.ResourceSetStatus{
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
								ID:      fmt.Sprintf("%s_my-app-config_%s_ConfigMap", ns.Name, ""),
								Version: "v1",
							},
						},
					},
					LastAppliedRevision: "sha256:abc123",
				}
				err = testClient.Status().Update(ctx, obj)
				g.Expect(err).ToNot(HaveOccurred())

				// Set up the resource argument with namespace
				kubeconfigArgs.Namespace = &ns.Name

				defer func() {
					_ = testClient.Delete(ctx, obj)
				}()
			}

			// Execute command
			output, err := executeCommand([]string{"export", "resource", tt.resourceArg})

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
			g.Expect(obj.GetKind()).To(Equal("ResourceSet"))
			g.Expect(obj.GetName()).To(Equal("config-template"))
			g.Expect(obj.GetNamespace()).ToNot(BeEmpty())

			// Verify spec content
			inputs, found, err := unstructured.NestedSlice(obj.Object, "spec", "inputs")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(inputs).To(HaveLen(1))

			// Verify the input content
			inputMap, ok := inputs[0].(map[string]any)
			g.Expect(ok).To(BeTrue())
			g.Expect(inputMap["appName"]).To(Equal("my-app"))
			g.Expect(inputMap["environment"]).To(Equal("staging"))

			// Verify resourcesTemplate
			template, found, err := unstructured.NestedString(obj.Object, "spec", "resourcesTemplate")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(template).To(ContainSubstring("kind: ConfigMap"))
			g.Expect(template).To(ContainSubstring("<< .inputs.appName >>"))
			g.Expect(template).To(ContainSubstring("<< .inputs.environment >>"))

			// Verify commonMetadata
			annotations, found, err := unstructured.NestedStringMap(obj.Object, "spec", "commonMetadata", "annotations")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(annotations).To(HaveKey("owner"))

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

func TestExportResourceCmdOutputFormat(t *testing.T) {
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

			// Create ResourceSet
			ns, err := testEnv.CreateNamespace(ctx, "test")
			g.Expect(err).ToNot(HaveOccurred())

			input, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
				"appName": "my-app",
			})
			g.Expect(err).ToNot(HaveOccurred())

			resourceSet := &fluxcdv1.ResourceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetSpec{
					Inputs:            []fluxcdv1.ResourceSetInput{input},
					ResourcesTemplate: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
				},
			}

			err = testClient.Create(ctx, resourceSet)
			g.Expect(err).ToNot(HaveOccurred())

			// Set up the namespace for the command
			kubeconfigArgs.Namespace = &ns.Name

			defer func() {
				_ = testClient.Delete(ctx, resourceSet)
			}()

			// Execute command with output format
			output, err := executeCommand([]string{"export", "resource", "ResourceSet/test-rs", "-o", tt.outputFormat})

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
				err = yaml.Unmarshal([]byte(output), &obj.Object)
			}
			g.Expect(err).ToNot(HaveOccurred())

			// Verify basic structure
			g.Expect(obj.GetAPIVersion()).To(Equal("fluxcd.controlplane.io/v1"))
			g.Expect(obj.GetKind()).To(Equal("ResourceSet"))
			g.Expect(obj.GetName()).To(Equal("test-rs"))

			// Verify spec content
			inputs, found, err := unstructured.NestedSlice(obj.Object, "spec", "inputs")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(inputs).To(HaveLen(1))
		})
	}
}
