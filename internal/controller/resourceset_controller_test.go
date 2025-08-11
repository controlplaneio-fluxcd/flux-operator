// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

func TestResourceSetReconciler_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  commonMetadata:
    annotations:
      owner: "%[1]s"
  inputs:
    - tenant: team1
    - tenant: team2
  resources:
    - apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: << inputs.tenant >>-readonly
        namespace: "%[1]s"
    - apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: << inputs.tenant >>-readwrite
        namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Check if the finalizer was added.
	resultInit := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultInit)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultInit)
	g.Expect(resultInit.Finalizers).To(ContainElement(fluxcdv1.Finalizer))

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the instance was installed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the inventory was updated.
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(4))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-readonly__ServiceAccount", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-readwrite__ServiceAccount", ns.Name),
			Version: "v1",
		},
	))

	// Check if the status last applied revision was set.
	g.Expect(result.Status.LastAppliedRevision).ToNot(BeEmpty())
	lastAppliedRevision := result.Status.LastAppliedRevision

	// Check if the history was updated.
	g.Expect(result.Status.History).To(HaveLen(1))
	g.Expect(result.Status.History[0].Digest).To(Equal(result.Status.LastAppliedRevision))
	g.Expect(result.Status.History[0].FirstReconciled).To(Equal(result.Status.History[0].LastReconciled))
	g.Expect(result.Status.History[0].LastReconciledDuration.Milliseconds()).To(BeNumerically(">", 0))
	g.Expect(result.Status.History[0].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("inputs", "2"))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("resources", "4"))

	// Check if the resources were created and labeled.
	resultSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team2-readwrite",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSA), resultSA)
	g.Expect(err).ToNot(HaveOccurred())

	expectedLabel := fmt.Sprintf("resourceset.%s", fluxcdv1.GroupVersion.Group)
	g.Expect(resultSA.Labels).To(HaveKeyWithValue(expectedLabel+"/name", "tenants"))
	g.Expect(resultSA.Labels).To(HaveKeyWithValue(expectedLabel+"/namespace", ns.Name))
	g.Expect(resultSA.Annotations).To(HaveKeyWithValue("owner", ns.Name))

	// Check if events were recorded for each step.
	events := getEvents(result.Name, result.Namespace)
	g.Expect(events).To(HaveLen(2))
	g.Expect(events[0].Reason).To(Equal("ApplySucceeded"))
	g.Expect(events[0].Message).To(ContainSubstring("team1-readonly created"))
	g.Expect(events[1].Reason).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(events[1].Message).To(HavePrefix("Reconciliation finished"))

	// Update the resource group.
	resultP := result.DeepCopy()
	resultP.SetAnnotations(map[string]string{
		fluxcdv1.ReconcileAnnotation:      fluxcdv1.EnabledValue,
		fluxcdv1.ReconcileEveryAnnotation: "1m",
	})
	resultP.Spec.Resources = resultP.Spec.Resources[:len(resultP.Spec.Resources)-1]

	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was scheduled for reconciliation.
	g.Expect(r.RequeueAfter).To(Equal(time.Minute))

	// Check the final status.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the inventory was updated.
	testutils.LogObject(t, resultFinal)
	g.Expect(resultFinal.Status.Inventory.Entries).To(HaveLen(2))
	g.Expect(resultFinal.Status.Inventory.Entries).ToNot(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-readwrite__ServiceAccount", ns.Name),
			Version: "v1",
		},
	))
	g.Expect(resultFinal.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-readonly__ServiceAccount", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-readonly__ServiceAccount", ns.Name),
			Version: "v1",
		},
	))

	// Check if the status last applied revision was updated.
	g.Expect(resultFinal.Status.LastAppliedRevision).ToNot(BeEmpty())
	g.Expect(resultFinal.Status.LastAppliedRevision).ToNot(BeEquivalentTo(lastAppliedRevision))

	// Check if the history was updated.
	g.Expect(resultFinal.Status.History).To(HaveLen(2))
	g.Expect(resultFinal.Status.History[0].Digest).To(Equal(resultFinal.Status.LastAppliedRevision))
	g.Expect(resultFinal.Status.History[1].Digest).To(Equal(result.Status.LastAppliedRevision))
	g.Expect(resultFinal.Status.History[0].Metadata).To(HaveKeyWithValue("resources", "2"))
	g.Expect(resultFinal.Status.History[1].Metadata).To(HaveKeyWithValue("resources", "4"))

	// Check if the resources were deleted.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSA), resultSA)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Delete the resource group.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the resource group was finalized.
	result = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetReconciler_CopyFrom(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  commonMetadata:
    annotations:
      owner: "%[1]s"
  inputs:
    - tenant: team1
    - tenant: team2
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.tenant >>
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/test-cm"
    - apiVersion: v1
      kind: Secret
      metadata:
        name: << inputs.tenant >>
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/test-secret"
    - apiVersion: v1
      kind: Secret
      metadata:
        name: << inputs.tenant >>-docker
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/test-secret-docker"
    - apiVersion: v1
      kind: Secret
      metadata:
        name: << inputs.tenant >>-keep-type
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/test-secret"
      type: CustomType
`, ns.Name)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"key": "value",
		},
	}
	err = testEnv.Create(ctx, cm)
	g.Expect(err).ToNot(HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"key": "value",
		},
	}
	err = testEnv.Create(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	dockerData := `{
	"auths": {
		"ghcr.io": {
			"auth": "dXNlcjpwYXNz"
		}
	}
}`
	secretDocker := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret-docker",
			Namespace: ns.Name,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			corev1.DockerConfigJsonKey: dockerData,
		},
	}
	err = testEnv.Create(ctx, secretDocker)
	g.Expect(err).ToNot(HaveOccurred())

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the ResourceSet.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile the ResourceSet.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the ResourceSet was deployed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the inventory was updated.
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(8))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1__Secret", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-docker__Secret", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team1-keep-type__Secret", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2__Secret", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-docker__Secret", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_team2-keep-type__Secret", ns.Name),
			Version: "v1",
		},
	))

	// Check if the resources were created with the copied data.
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team1",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultCM.Annotations).To(HaveKeyWithValue("owner", ns.Name))
	g.Expect(resultCM.Data).To(HaveKeyWithValue("key", "value"))

	resultSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team2",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSecret), resultSecret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultSecret.Annotations).To(HaveKeyWithValue("owner", ns.Name))
	g.Expect(resultSecret.Data).To(HaveKeyWithValue("key", []byte("value")))

	resultSecretDocker := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team2-docker",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSecretDocker), resultSecretDocker)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultSecretDocker.Annotations).To(HaveKeyWithValue("owner", ns.Name))
	g.Expect(resultSecretDocker.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
	g.Expect(resultSecretDocker.Data).To(HaveKeyWithValue(corev1.DockerConfigJsonKey, []byte(dockerData)))

	resultSecretCustomType := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "team2-keep-type",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSecretCustomType), resultSecretCustomType)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultSecretCustomType.Annotations).To(HaveKeyWithValue("owner", ns.Name))
	g.Expect(resultSecretCustomType.Type).To(Equal(corev1.SecretType("CustomType")))
	g.Expect(resultSecretCustomType.Data).To(HaveKeyWithValue("key", []byte("value")))

	// Update the source ConfigMap.
	cm.Data = map[string]string{"key1": "updated1"}
	err = testClient.Update(ctx, cm)
	g.Expect(err).ToNot(HaveOccurred())

	// Update the source Secret.
	secret.Data["key"] = []byte("updated")
	err = testClient.Update(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the ResourceSet.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ConfigMap was updated.
	finalCM := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), finalCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(finalCM.Data).NotTo(HaveKeyWithValue("key", "value"))
	g.Expect(finalCM.Data).To(HaveKeyWithValue("key1", "updated1"))

	// Check if the Secret was updated.
	finalSecret := &corev1.Secret{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSecret), finalSecret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(finalSecret.Data).To(HaveKeyWithValue("key", []byte("updated")))

	// Delete the resource group.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())

	// Check if the resource group was finalized.
	result = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

	// Check if the resources were deleted.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultSecret), resultSecret)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetReconciler_DependsOn(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  dependsOn:
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: fluxinstances.fluxcd.controlplane.io
      ready: true
      readyExpr: |
        status.conditions.filter(e, e.type == 'Established').all(e, e.status == 'True') &&
        status.storedVersions.exists(e, e =='v1')
    - apiVersion: v1
      kind: ServiceAccount
      name: test
      namespace: "%[1]s"
  resources:
    - apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: readonly
        namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile with not found dependency.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.RequeueAfter).To(Equal(5 * time.Second))

	// Check if the instance was installed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.DependencyNotReadyReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring("\"test\" not found"))

	// Create the dependency.
	dep := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		},
	}

	err = testClient.Create(ctx, dep)
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with ready dependencies.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was installed.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Delete the resource group.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetReconciler_DependsOnInvalidExpression(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  dependsOn:
    - apiVersion: apiextensions.k8s.io/v1
      kind: CustomResourceDefinition
      name: fluxinstances.fluxcd.controlplane.io
      ready: true
      readyExpr: status.
    - apiVersion: v1
      kind: ServiceAccount
      name: test
      namespace: "%[1]s"
  resources:
    - apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: readonly
        namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile with invalid expression.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())

	// Check if the instance was installed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.IsStalled(result)).To(BeTrue())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.InvalidCELExpressionReason))
	g.Expect(conditions.GetMessage(result, meta.ReadyCondition)).To(ContainSubstring("failed to parse expression"))
}

func TestResourceSetInputsFromValidation(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Both set.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			InputsFrom: []fluxcdv1.InputProviderReference{{
				Kind:     fluxcdv1.ResourceSetInputProviderKind,
				Name:     "test",
				Selector: &metav1.LabelSelector{},
			}},
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot set both name and selector for input provider references"))

	// Neither set.
	err = testEnv.Create(ctx, &fluxcdv1.ResourceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetSpec{
			InputsFrom: []fluxcdv1.InputProviderReference{{
				Kind: fluxcdv1.ResourceSetInputProviderKind,
			}},
		},
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("at least one of name or selector must be set for input provider references"))
}

func TestResourceSetReconciler_LabelSelector(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	rsipReconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	rsipZeroDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-0
  namespace: "%[1]s"
spec:
  type: Static
  defaultValues:
    foo: app-0-foo
    baz: app-0-baz
`, ns.Name)

	rsipOneDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-1
  namespace: "%[1]s"
  labels:
    app: app-1
    my: tenant
spec:
  type: Static
  defaultValues:
    foo: bar
    baz: qux
`, ns.Name)

	rsipTwoDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: app-2
  namespace: "%[1]s"
  labels:
    app: app-2
    my: tenant
spec:
  type: Static
  defaultValues:
    foo: qux
    baz: bar
`, ns.Name)

	// Create, initialize and reconcile the ResourceSetInputProviders.
	rsipID := make([]string, 3)
	for i, def := range []string{rsipZeroDef, rsipOneDef, rsipTwoDef} {
		obj := &fluxcdv1.ResourceSetInputProvider{}
		err = yaml.Unmarshal([]byte(def), obj)
		g.Expect(err).NotTo(HaveOccurred())
		err = testEnv.Create(ctx, obj)
		g.Expect(err).NotTo(HaveOccurred())
		r, err := rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(r.Requeue).To(BeTrue())
		r, err = rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(r.Requeue).To(BeFalse())
		result := &fluxcdv1.ResourceSetInputProvider{}
		err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsReady(result)).To(BeTrue())
		rsipID[i] = inputs.ID(string(result.GetUID()))
	}
	rsipZeroID := rsipID[0]
	rsipOneID := rsipID[1]
	rsipTwoID := rsipID[2]

	rsetDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  inputs:
    - id: inputs-dont-have-a-default-id
      foo: rset-foo
      baz: rset-baz
  inputsFrom:
    - kind: ResourceSetInputProvider
      name: app-0
    - kind: ResourceSetInputProvider # this tests deduplication
      selector:
        matchLabels:
          my: tenant
    - kind: ResourceSetInputProvider
      selector:
        matchExpressions:
          - key: app
            operator: In
            values:
              - app-1
              - app-2
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: cm-<< inputs.id >>
        namespace: "%[1]s"
      data:
        providerAPIVersion: << inputs.provider.apiVersion >>
        providerKind: << inputs.provider.kind >>
        providerName: << inputs.provider.name >>
        providerNamespace: << inputs.provider.namespace >>
        foo: << inputs.foo >>
        baz: << inputs.baz >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(rsetDef), obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize and reconcile the ResourceSet.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())

	// Assert inventory entries.
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(4))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-inputs-dont-have-a-default-id__ConfigMap", ns.Name),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipZeroID),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipOneID),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipTwoID),
			Version: "v1",
		},
	))

	// Get ConfigMaps and assert data.
	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      "cm-inputs-dont-have-a-default-id",
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cm.Data["providerAPIVersion"]).To(Equal("fluxcd.controlplane.io/v1"))
	g.Expect(cm.Data["providerKind"]).To(Equal("ResourceSet"))
	g.Expect(cm.Data["providerName"]).To(Equal("tenants"))
	g.Expect(cm.Data["providerNamespace"]).To(Equal(ns.Name))
	g.Expect(cm.Data["foo"]).To(Equal("rset-foo"))
	g.Expect(cm.Data["baz"]).To(Equal("rset-baz"))
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipZeroID),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cm.Data["providerAPIVersion"]).To(Equal("fluxcd.controlplane.io/v1"))
	g.Expect(cm.Data["providerKind"]).To(Equal("ResourceSetInputProvider"))
	g.Expect(cm.Data["providerName"]).To(Equal("app-0"))
	g.Expect(cm.Data["providerNamespace"]).To(Equal(ns.Name))
	g.Expect(cm.Data["foo"]).To(Equal("app-0-foo"))
	g.Expect(cm.Data["baz"]).To(Equal("app-0-baz"))
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipOneID),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cm.Data["providerAPIVersion"]).To(Equal("fluxcd.controlplane.io/v1"))
	g.Expect(cm.Data["providerKind"]).To(Equal("ResourceSetInputProvider"))
	g.Expect(cm.Data["providerName"]).To(Equal("app-1"))
	g.Expect(cm.Data["providerNamespace"]).To(Equal(ns.Name))
	g.Expect(cm.Data["foo"]).To(Equal("bar"))
	g.Expect(cm.Data["baz"]).To(Equal("qux"))
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipTwoID),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cm.Data["providerAPIVersion"]).To(Equal("fluxcd.controlplane.io/v1"))
	g.Expect(cm.Data["providerKind"]).To(Equal("ResourceSetInputProvider"))
	g.Expect(cm.Data["providerName"]).To(Equal("app-2"))
	g.Expect(cm.Data["providerNamespace"]).To(Equal(ns.Name))
	g.Expect(cm.Data["foo"]).To(Equal("qux"))
	g.Expect(cm.Data["baz"]).To(Equal("bar"))
}

func TestResourceSetReconciler_LabelSelector_LifeCycle(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	rsipReconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	rsipFmt := `
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    my: tenant
spec:
  type: Static
  defaultValues:
    foo: qux
    baz: bar
`

	// RSIP helpers.
	createRSIP := func(name string) string {
		obj := &fluxcdv1.ResourceSetInputProvider{}
		err = yaml.Unmarshal([]byte(fmt.Sprintf(rsipFmt, name, ns.Name)), obj)
		g.Expect(err).NotTo(HaveOccurred())
		err = testEnv.Create(ctx, obj)
		g.Expect(err).NotTo(HaveOccurred())
		r, err := rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(r.Requeue).To(BeTrue())
		r, err = rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(obj),
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(r.Requeue).To(BeFalse())
		result := &fluxcdv1.ResourceSetInputProvider{}
		err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsReady(result)).To(BeTrue())
		return inputs.ID(string(result.GetUID()))
	}
	deleteRSIP := func(name string) {
		err := testClient.Delete(ctx, &fluxcdv1.ResourceSetInputProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns.Name,
			},
		})
		g.Expect(err).NotTo(HaveOccurred())
		r, err := rsipReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      name,
				Namespace: ns.Name,
			},
		})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(r.Requeue).To(BeFalse())
		g.Eventually(func() bool {
			err = testClient.Get(ctx, client.ObjectKey{
				Name:      name,
				Namespace: ns.Name,
			}, &fluxcdv1.ResourceSetInputProvider{})
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	}

	rsetDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: tenants
  namespace: "%[1]s"
spec:
  inputsFrom:
    - kind: ResourceSetInputProvider
      selector:
        matchLabels:
          my: tenant
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: cm-<< inputs.id >>
        namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(rsetDef), obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize and reconcile the ResourceSet.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())

	// Assert empty inventory.
	g.Expect(result.Status.Inventory.Entries).To(BeEmpty())

	// Create two RSIPs, reconcile RSET and check inventory.
	rsipOne := createRSIP("app-1")
	rsipTwo := createRSIP("app-2")
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(2))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipOne),
			Version: "v1",
		},
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipTwo),
			Version: "v1",
		},
	))
	cm := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipOne),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipTwo),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).NotTo(HaveOccurred())

	// Delete RSIP one, reconcile RSET and check inventory.
	deleteRSIP("app-1")
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(1))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_cm-%s__ConfigMap", ns.Name, rsipTwo),
			Version: "v1",
		},
	))

	// Delete RSIP two, reconcile RSET and check inventory.
	deleteRSIP("app-2")
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())
	result = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())
	g.Expect(result.Status.Inventory.Entries).To(BeEmpty())
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipTwo),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	cm = &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cm-%s", rsipOne),
		Namespace: ns.Name,
	}, cm)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetReconciler_Impersonation(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test
  namespace: "%[1]s"
spec:
  serviceAccountName: flux-operator
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: test
        namespace: "%[1]s"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile with missing service account.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())

	// Check if the instance was installed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))

	// Create the service account and role binding.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-operator",
			Namespace: ns.Name,
		},
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux-operator",
			Namespace: ns.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "flux-operator",
				Namespace: ns.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "cluster-admin",
		},
	}

	err = testClient.Create(ctx, sa)
	g.Expect(err).ToNot(HaveOccurred())
	err = testClient.Create(ctx, rb)
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with existing service account.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the instance was installed.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Delete the resource group.
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.IsZero()).To(BeTrue())
}

func TestResourceSetReconciler_HistoryErrorTracking(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Start with a working ResourceSet
	objDefWorking := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: error-tracking-test
  namespace: "%[1]s"
spec:
  inputs:
    - tenant: team1
  resourcesTemplate: |
    apiVersion: v1
    kind: ServiceAccount
    metadata:
     name: << inputs.tenant >>-readonly
     namespace: << inputs.provider.namespace >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDefWorking), obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize the instance.
	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile the working ResourceSet.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check if the ResourceSet was deployed successfully.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, result)

	// Check if the ready condition is set to true.
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the history has one successful entry.
	g.Expect(result.Status.History).To(HaveLen(1))
	g.Expect(result.Status.History[0].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("resources", "1"))
	g.Expect(result.Status.History[0].Metadata).To(HaveKeyWithValue("inputs", "1"))

	// Update to cause a build error (invalid Go template function)
	resultP := result.DeepCopy()
	resultP.Spec.ResourcesTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: << invalidFunc inputs.tenant >>-readonly
  namespace: << inputs.provider.namespace >>`

	err = testClient.Patch(ctx, resultP, client.MergeFrom(result))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with build error
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, reconcile.TerminalError(nil))).To(BeTrue())

	// Check the build error result.
	resultBuildError := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultBuildError)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultBuildError)

	// Check if the ready condition is set to false with build failed reason.
	g.Expect(conditions.IsStalled(resultBuildError)).To(BeTrue())
	g.Expect(conditions.GetReason(resultBuildError, meta.ReadyCondition)).To(BeIdenticalTo(meta.BuildFailedReason))

	// Check if the history has two entries - build error should be first (most recent)
	g.Expect(resultBuildError.Status.History).To(HaveLen(2))
	g.Expect(resultBuildError.Status.History[0].LastReconciledStatus).To(Equal(meta.BuildFailedReason))
	g.Expect(resultBuildError.Status.History[1].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))

	// Update to cause an apply error (non-existing kind)
	resultP2 := resultBuildError.DeepCopy()
	resultP2.Spec.ResourcesTemplate = `
apiVersion: example.com/v1
kind: NonExistentKind
metadata:
  name: << inputs.tenant >>-test
  namespace: << inputs.provider.namespace >>
spec:
  data: test`

	err = testClient.Patch(ctx, resultP2, client.MergeFrom(resultBuildError))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with apply error
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())

	// Check the apply error result.
	resultApplyError := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultApplyError)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultApplyError)

	// Check if the ready condition is set to false with reconciliation failed reason.
	g.Expect(conditions.IsReady(resultApplyError)).To(BeFalse())
	g.Expect(conditions.GetReason(resultApplyError, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationFailedReason))

	// Check if the history has three entries - apply error should be first (most recent)
	g.Expect(resultApplyError.Status.History).To(HaveLen(3))
	g.Expect(resultApplyError.Status.History[0].LastReconciledStatus).To(Equal(meta.ReconciliationFailedReason))
	g.Expect(resultApplyError.Status.History[1].LastReconciledStatus).To(Equal(meta.BuildFailedReason))
	g.Expect(resultApplyError.Status.History[2].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))

	// Update back to working spec to verify successful reconciliation gets added to history
	resultP3 := resultApplyError.DeepCopy()
	resultP3.Spec.ResourcesTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: << inputs.tenant >>-readonly
  namespace: << inputs.provider.namespace >>`

	err = testClient.Patch(ctx, resultP3, client.MergeFrom(resultApplyError))
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile with working spec
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Check the final working result.
	resultFinal := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), resultFinal)
	g.Expect(err).ToNot(HaveOccurred())

	testutils.LogObjectStatus(t, resultFinal)

	// Check if the ready condition is set to true with reconciliation succeeded reason.
	g.Expect(conditions.IsReady(resultFinal)).To(BeTrue())
	g.Expect(conditions.GetReason(resultFinal, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Check if the history has three entries - working spec should move existing entry to front
	g.Expect(resultFinal.Status.History).To(HaveLen(3))
	g.Expect(resultFinal.Status.History[0].Digest).To(Equal(resultFinal.Status.LastAppliedRevision))
	g.Expect(resultFinal.Status.History[0].LastReconciledStatus).To(Equal(meta.ReconciliationSucceededReason))
	g.Expect(resultFinal.Status.History[0].TotalReconciliations).To(BeEquivalentTo(2))
	g.Expect(resultFinal.Status.History[1].LastReconciledStatus).To(Equal(meta.ReconciliationFailedReason))
	g.Expect(resultFinal.Status.History[1].TotalReconciliations).To(BeEquivalentTo(1))
	g.Expect(resultFinal.Status.History[2].LastReconciledStatus).To(Equal(meta.BuildFailedReason))
	g.Expect(resultFinal.Status.History[2].TotalReconciliations).To(BeEquivalentTo(1))

	// Clean up
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func getResourceSetReconciler(t *testing.T) *ResourceSetReconciler {
	tmpDir := t.TempDir()
	err := os.WriteFile(fmt.Sprintf("%s/kubeconfig", tmpDir), testKubeConfig, 0644)
	if err != nil {
		panic(fmt.Sprintf("failed to create the testenv-admin user kubeconfig: %v", err))
	}

	// Set the kubeconfig environment variable for the impersonator.
	t.Setenv("KUBECONFIG", fmt.Sprintf("%s/kubeconfig", tmpDir))

	// Disable notifications for the tests as no pod is running.
	// This is required to avoid the 30s retry loop performed by the HTTP client.
	t.Setenv("NOTIFICATIONS_DISABLED", "yes")

	return &ResourceSetReconciler{
		Client:            testClient,
		APIReader:         testClient,
		Scheme:            NewTestScheme(),
		StatusManager:     controllerName,
		EventRecorder:     testEnv.GetEventRecorderFor(controllerName),
		RequeueDependency: 5 * time.Second,
	}
}
