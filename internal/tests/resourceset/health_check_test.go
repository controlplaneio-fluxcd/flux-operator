// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package resourceset

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func TestResourceSetReconciler_HealthCheckCanceled(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a ResourceSet with Wait enabled that will trigger health checks.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test-health-check
  namespace: "%[1]s"
  annotations:
    # Set a long timeout so health checks don't fail before we can cancel them.
    fluxcd.controlplane.io/reconcile-timeout: "5m"
spec:
  wait: true
  inputs:
    - name: test
  resources:
    # Create a Deployment that won't become ready because the image doesn't exist.
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: << inputs.name >>-deployment
        namespace: "%[1]s"
      spec:
        replicas: 1
        selector:
          matchLabels:
            app: << inputs.name >>
        template:
          metadata:
            labels:
              app: << inputs.name >>
          spec:
            containers:
              - name: app
                image: nonexistent-image:v999
                imagePullPolicy: Never
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Disable notifications for the tests.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	// Set the kubeconfig environment variable for the impersonator.
	tmpDir := t.TempDir()
	err = os.WriteFile(fmt.Sprintf("%s/kubeconfig", tmpDir), testKubeConfig, 0644)
	g.Expect(err).ToNot(HaveOccurred())
	t.Setenv("KUBECONFIG", fmt.Sprintf("%s/kubeconfig", tmpDir))

	err = testClient.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait until the instance has started reconciling (finalizer added and Reconciling condition set).
	// Note: The inventory is only set after health checks complete, so we can't check for it here.
	g.Eventually(func() bool {
		result := &fluxcdv1.ResourceSet{}
		if err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), result); err != nil {
			return false
		}
		// The instance should have finalizer and be in Reconciling state.
		return len(result.Finalizers) > 0 &&
			conditions.Has(result, meta.ReconcilingCondition) &&
			conditions.GetReason(result, meta.ReconcilingCondition) == meta.ProgressingReason
	}, 60*time.Second, 100*time.Millisecond).Should(BeTrue(), "instance should have started reconciling")

	// Give the controller a moment to start health checks after the condition is set.
	// The health checks run inside apply() after the deployment is created.
	time.Sleep(500 * time.Millisecond)

	// Trigger a new reconciliation by updating the spec (adding a new resource).
	// This should cause the health check to be canceled and a new reconciliation to start.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	resultP := result.DeepCopy()
	// Add an additional resource that will change the inventory.
	resultP.Spec.Resources = append(resultP.Spec.Resources, &apiextensionsv1.JSON{
		Raw: fmt.Appendf(nil, `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-configmap","namespace":"%s"},"data":{"key":"value"}}`, ns.Name),
	})
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
	g.Expect(cancelEvent.Message).To(ContainSubstring("ResourceSet"))

	// Clean up.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Wait for the finalizer to be removed.
	g.Eventually(func() bool {
		err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), &fluxcdv1.ResourceSet{})
		return err != nil
	}, 60*time.Second, 1*time.Second).Should(BeTrue(), "instance should be deleted")
}
