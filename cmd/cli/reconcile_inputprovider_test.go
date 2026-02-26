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

func TestReconcileInputProviderCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wait        bool
		expectError bool
	}{
		{
			name: "reconcile inputprovider without wait",
			args: []string{"reconcile", "inputprovider", "test-provider", "--wait=false"},
		},
		{
			name: "reconcile inputprovider with wait",
			args: []string{"reconcile", "inputprovider", "test-provider", "--wait"},
			wait: true,
		},
		{
			name: "reconcile with force",
			args: []string{"reconcile", "inputprovider", "test-provider", "--force", "--wait=false"},
		},
		{
			name: "reconcile using alias",
			args: []string{"reconcile", "rsip", "test-provider", "--wait=false"},
		},
		{
			name:        "missing name",
			args:        []string{"reconcile", "inputprovider"},
			expectError: true,
		},
		{
			name:        "non-existent inputprovider",
			args:        []string{"reconcile", "inputprovider", "nonexistent", "--wait=false"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			ns, err := testEnv.CreateNamespace(ctx, "test-reconcile-ip")
			g.Expect(err).ToNot(HaveOccurred())

			defaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
				"env": "staging",
			})
			g.Expect(err).ToNot(HaveOccurred())

			provider := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-provider",
					Namespace: ns.Name,
				},
				Spec: fluxcdv1.ResourceSetInputProviderSpec{
					Type:          fluxcdv1.InputProviderStatic,
					DefaultValues: defaultValues,
				},
			}
			err = testClient.Create(ctx, provider)
			g.Expect(err).ToNot(HaveOccurred())
			defer func() {
				_ = testClient.Delete(ctx, provider)
			}()

			kubeconfigArgs.Namespace = &ns.Name

			if tt.wait {
				simulateReconciliation(ctx,
					types.NamespacedName{Name: "test-provider", Namespace: ns.Name},
					&fluxcdv1.ResourceSetInputProvider{})
			}

			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(output).To(ContainSubstring("triggered"))

			updated := &fluxcdv1.ResourceSetInputProvider{}
			err = testClient.Get(ctx, types.NamespacedName{Name: "test-provider", Namespace: ns.Name}, updated)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))

			if tt.name == "reconcile with force" {
				g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ForceRequestAnnotation))
			}

			if tt.wait {
				g.Expect(output).To(ContainSubstring("Reconciliation succeeded"))
			}
		})
	}
}
