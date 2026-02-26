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

func TestResumeInputProviderCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name: "resume inputprovider without wait",
			args: []string{"resume", "inputprovider", "test-provider", "--wait=false"},
		},
		{
			name: "resume using alias",
			args: []string{"resume", "rsip", "test-provider", "--wait=false"},
		},
		{
			name:        "missing name",
			args:        []string{"resume", "inputprovider"},
			expectError: true,
		},
		{
			name:        "non-existent inputprovider",
			args:        []string{"resume", "inputprovider", "nonexistent", "--wait=false"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			ns, err := testEnv.CreateNamespace(ctx, "test-resume-ip")
			g.Expect(err).ToNot(HaveOccurred())

			defaultValues, err := fluxcdv1.NewResourceSetInput(nil, map[string]any{
				"env": "staging",
			})
			g.Expect(err).ToNot(HaveOccurred())

			provider := &fluxcdv1.ResourceSetInputProvider{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-provider",
					Namespace: ns.Name,
					Annotations: map[string]string{
						fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue,
					},
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

			output, err := executeCommand(tt.args)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())

			updated := &fluxcdv1.ResourceSetInputProvider{}
			err = testClient.Get(ctx, types.NamespacedName{Name: "test-provider", Namespace: ns.Name}, updated)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updated.GetAnnotations()).To(HaveKeyWithValue(fluxcdv1.ReconcileAnnotation, fluxcdv1.EnabledValue))
			g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))

			g.Expect(output).To(ContainSubstring("resumed"))
		})
	}
}
