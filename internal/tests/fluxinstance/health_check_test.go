// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package fluxinstance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestFluxInstanceReconciler_HealthCheckCanceled(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a FluxInstance with Wait enabled that will trigger health checks.
	// Set replicas to 0 so deployments won't become ready, keeping the health check running.
	obj := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: ns.Name,
			Annotations: map[string]string{
				// Set a long timeout so health checks don't fail before we can cancel them.
				fluxcdv1.ReconcileTimeoutAnnotation: "5m",
			},
		},
		Spec: fluxcdv1.FluxInstanceSpec{
			Wait:             ptr.To(true),
			MigrateResources: ptr.To(false),
			Distribution: fluxcdv1.Distribution{
				Version:  "v2.3.0",
				Registry: "ghcr.io/fluxcd",
			},
			Components: []fluxcdv1.Component{"source-controller"},
			Kustomize: &fluxcdv1.Kustomize{
				Patches: []kustomize.Patch{
					{
						Target: &kustomize.Selector{
							Kind: "Deployment",
						},
						// Set replicas to 0 so the deployment won't become ready,
						// keeping the health check running indefinitely.
						Patch: `
- op: replace
  path: /spec/replicas
  value: 0
`,
					},
				},
			},
		},
	}

	// Disable notifications for the tests.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	// Set the kubeconfig environment variable for the impersonator.
	tmpDir := t.TempDir()
	err = os.WriteFile(fmt.Sprintf("%s/kubeconfig", tmpDir), testKubeConfig, 0644)
	g.Expect(err).ToNot(HaveOccurred())
	t.Setenv("KUBECONFIG", fmt.Sprintf("%s/kubeconfig", tmpDir))

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait until the instance is in health check phase (Reconciling with ProgressingReason
	// and has attempted a revision).
	g.Eventually(func() bool {
		result := &fluxcdv1.FluxInstance{}
		if err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), result); err != nil {
			return false
		}
		// The instance should be reconciling and have applied resources.
		return conditions.Has(result, meta.ReconcilingCondition) &&
			conditions.GetReason(result, meta.ReconcilingCondition) == meta.ProgressingReason &&
			result.Status.LastAttemptedRevision != ""
	}, 60*time.Second, 1*time.Second).Should(BeTrue(), "instance should be in health check phase")

	// Record the current revision before the spec change.
	result := &fluxcdv1.FluxInstance{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	originalRevision := result.Status.LastAttemptedRevision
	t.Logf("Original revision: %s", originalRevision)

	// Trigger a new reconciliation by updating the spec.
	// This should cause the health check to be canceled and a new reconciliation to start.
	resultP := result.DeepCopy()
	resultP.Spec.Distribution.Registry = "docker.io/fluxcd"
	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for the HealthCheckCanceled event to be recorded.
	// This event is emitted when health checks are canceled due to a new reconciliation.
	g.Eventually(func() bool {
		events := getEvents(obj.Name, obj.Namespace)
		for _, event := range events {
			if event.Reason == meta.HealthCheckCanceledReason {
				t.Logf("Found HealthCheckCanceled event: %s", event.Message)
				return true
			}
		}
		return false
	}, 30*time.Second, 100*time.Millisecond).Should(BeTrue(), "HealthCheckCanceled event should be recorded")

	// Verify the event message indicates the trigger source.
	events := getEvents(obj.Name, obj.Namespace)
	var cancelEvent *corev1.Event
	for i := range events {
		if events[i].Reason == meta.HealthCheckCanceledReason {
			cancelEvent = &events[i]
			break
		}
	}
	g.Expect(cancelEvent).ToNot(BeNil())
	g.Expect(cancelEvent.Message).To(ContainSubstring("Health checks canceled"))
	g.Expect(cancelEvent.Message).To(ContainSubstring("FluxInstance"))

	// Clean up.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for the finalizer to be removed.
	g.Eventually(func() bool {
		err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), &fluxcdv1.FluxInstance{})
		return err != nil
	}, 60*time.Second, 1*time.Second).Should(BeTrue(), "instance should be deleted")
}
