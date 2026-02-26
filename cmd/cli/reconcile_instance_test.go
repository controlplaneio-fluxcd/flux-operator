// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// simulateReconciliation watches for a reconcile request annotation on the given
// resource and patches its status to simulate a successful reconciliation.
// It must be called before executeCommand so the goroutine can respond to the annotation.
func simulateReconciliation(ctx context.Context, key types.NamespacedName, obj fluxcdv1.FluxObject) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := testClient.Get(ctx, key, obj); err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			requestTime := obj.GetAnnotations()[meta.ReconcileRequestAnnotation]
			if requestTime == "" {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			obj.SetLastHandledReconcileAt(requestTime)
			obj.SetConditions([]metav1.Condition{
				{
					Type:               meta.ReadyCondition,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: obj.GetGeneration(),
					LastTransitionTime: metav1.Now(),
					Reason:             meta.ReconciliationSucceededReason,
					Message:            "Reconciliation succeeded",
				},
			})
			if err := testClient.Status().Update(ctx, obj); err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}
	}()
}

func TestReconcileInstanceCmd(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wait        bool
		expectError bool
	}{
		{
			name: "reconcile instance without wait",
			args: []string{"reconcile", "instance", "flux", "--wait=false"},
		},
		{
			name: "reconcile instance with wait",
			args: []string{"reconcile", "instance", "flux", "--wait"},
			wait: true,
		},
		{
			name: "reconcile instance with force",
			args: []string{"reconcile", "instance", "flux", "--force", "--wait=false"},
		},
		{
			name:        "missing name",
			args:        []string{"reconcile", "instance"},
			expectError: true,
		},
		{
			name:        "non-existent instance",
			args:        []string{"reconcile", "instance", "nonexistent", "--wait=false"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			g := NewWithT(t)

			ns, err := testEnv.CreateNamespace(ctx, "test-reconcile-inst")
			g.Expect(err).ToNot(HaveOccurred())

			instance := &fluxcdv1.FluxInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "flux",
					Namespace: ns.Name,
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
			g.Expect(output).To(ContainSubstring("triggered"))

			// Verify the annotation was set.
			updated := &fluxcdv1.FluxInstance{}
			err = testClient.Get(ctx, types.NamespacedName{Name: "flux", Namespace: ns.Name}, updated)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ReconcileRequestAnnotation))

			if tt.name == "reconcile instance with force" {
				g.Expect(updated.GetAnnotations()).To(HaveKey(meta.ForceRequestAnnotation))
			}

			if tt.wait {
				g.Expect(output).To(ContainSubstring("Reconciliation succeeded"))
			}
		})
	}
}

func TestReconcileInstanceCmd_Suspended(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	g := NewWithT(t)

	ns, err := testEnv.CreateNamespace(ctx, "test-reconcile-susp")
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

	// Reconcile with wait on a suspended instance should fail.
	_, err = executeCommand([]string{"reconcile", "instance", "flux", "--wait", "--timeout=3s"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("disabled"))
}
