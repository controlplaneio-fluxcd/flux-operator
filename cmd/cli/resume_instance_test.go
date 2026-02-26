// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestResumeInstanceCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wait        bool
		expectError bool
	}{
		{
			name: "resume instance without wait",
			args: []string{"resume", "instance", "flux", "--wait=false"},
		},
		{
			name: "resume instance with wait",
			args: []string{"resume", "instance", "flux", "--wait"},
			wait: true,
		},
		{
			name:        "missing name",
			args:        []string{"resume", "instance"},
			expectError: true,
		},
		{
			name:        "non-existent instance",
			args:        []string{"resume", "instance", "nonexistent", "--wait=false"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			ns, err := testEnv.CreateNamespace(ctx, "test-resume-inst")
			g.Expect(err).ToNot(HaveOccurred())

			instance := &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "flux",
					Namespace: ns.Name,
					Annotations: map[string]string{
						fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
					},
				},
				Spec: fluxcdv1.FluxInstanceSpec{
					Distribution: fluxcdv1.Distribution{
						Version:  "v2.x",
						Registry: "ghcr.io/fluxcd",
						Artifact: "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
					},
				},
			}
			err = testClient.Create(ctx, instance)
			g.Expect(err).ToNot(HaveOccurred())
			defer func() {
				_ = testClient.Delete(ctx, instance)
			}()

			kubeconfigArgs.Namespace = &ns.Name

			if tt.wait {
				simulateReconciliation(ctx,
					types.NamespacedName{Name: "flux", Namespace: ns.Name},
					&fluxcdv1.FluxInstance{})
			}

			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			// Verify the annotation was changed from disabled to enabled.
			updated := &fluxcdv1.FluxInstance{}
			err = testClient.Get(ctx, types.NamespacedName{Name: "flux", Namespace: ns.Name}, updated)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updated.GetAnnotations()).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.EnabledValue))
			g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))

			if tt.wait {
				g.Expect(output).To(ContainSubstring("Reconciliation succeeded"))
			} else {
				g.Expect(output).To(ContainSubstring("resumed"))
			}
		})
	}
}
