// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestTreeResourceSetCmd(t *testing.T) {
	gt := NewWithT(t)
	ns, err := testEnv.CreateNamespace(context.Background(), "tree-rset")
	gt.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name           string
		setupResources bool
		resourceName   string
		expectError    bool
		expectOutput   string
	}{
		{
			name:        "missing resource name argument",
			expectError: true,
		},
		{
			name:         "nonexistent resource",
			resourceName: "nonexistent",
			expectError:  true,
		},
		{
			name:           "resourceset with configmap and serviceaccount",
			setupResources: true,
			resourceName:   "test-app",
			expectOutput: fmt.Sprintf(`
ResourceSet/%[1]s/test-app
├── ServiceAccount/%[1]s/test-serviceaccount
└── ConfigMap/%[1]s/test-configmap`, ns.Name),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			var resourceSet *fluxcdv1.ResourceSet

			// Setup resources if needed
			if tt.setupResources {
				// Create a ResourceSet
				input, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
					"app": "test-app",
				})
				g.Expect(err).ToNot(HaveOccurred())

				resourceSet = &fluxcdv1.ResourceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: ns.Name,
					},
					Spec: fluxcdv1.ResourceSetSpec{
						Inputs:            []fluxcdv1.ResourceSetInput{input},
						ResourcesTemplate: "---",
					},
				}
				err = testClient.Create(ctx, resourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				// Set status with inventory entries for the managed resources
				resourceSet.Status = fluxcdv1.ResourceSetStatus{
					Inventory: &fluxcdv1.ResourceInventory{
						Entries: []fluxcdv1.ResourceRef{
							{
								ID:      fmt.Sprintf("%s_test-serviceaccount__ServiceAccount", ns.Name),
								Version: "v1",
							},
							{
								ID:      fmt.Sprintf("%s_test-configmap__ConfigMap", ns.Name),
								Version: "v1",
							},
						},
					},
				}
				err = testClient.Status().Update(ctx, resourceSet)
				g.Expect(err).ToNot(HaveOccurred())

				defer func() {
					_ = testClient.Delete(ctx, resourceSet)
				}()

				// Set namespace for command
				kubeconfigArgs.Namespace = &ns.Name
			}

			// Prepare command arguments
			args := []string{"tree", "resourceset"}
			if tt.resourceName != "" {
				args = append(args, tt.resourceName)
			}

			// Execute command
			output, err := executeCommand(args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			// Verify expected output
			g.Expect(strings.TrimSpace(output)).To(BeIdenticalTo(strings.TrimSpace(tt.expectOutput)))
		})
	}
}
