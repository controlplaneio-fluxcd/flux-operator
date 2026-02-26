// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestSuspendResourceSetCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name: "suspend resourceset",
			args: []string{"suspend", "resourceset", "test-rs"},
		},
		{
			name:        "missing name",
			args:        []string{"suspend", "resourceset"},
			expectError: true,
		},
		{
			name:        "non-existent resourceset",
			args:        []string{"suspend", "resourceset", "nonexistent"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			ns, err := testEnv.CreateNamespace(ctx, "test-suspend-rs")
			g.Expect(err).ToNot(HaveOccurred())

			input, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
				"app": "test",
			})
			g.Expect(err).ToNot(HaveOccurred())

			rs := &fluxcdv1.ResourceSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
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
			defer func() {
				_ = testClient.Delete(ctx, rs)
			}()

			kubeconfigArgs.Namespace = &ns.Name
			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(ContainSubstring("suspended"))

			// Verify the annotation was set.
			updated := &fluxcdv1.ResourceSet{}
			err = testClient.Get(ctx, types.NamespacedName{Name: "test-rs", Namespace: ns.Name}, updated)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updated.GetAnnotations()).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.DisabledValue))
		})
	}
}
