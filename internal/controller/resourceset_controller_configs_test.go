// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/testutils"
)

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

func TestResourceSetReconciler_CopyFromFieldRemoval(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create source Secret with two fields.
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-secret",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	// Create source ConfigMap with two fields.
	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-cm",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	err = testEnv.Create(ctx, sourceCM)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test-field-removal
  namespace: "%[1]s"
spec:
  resources:
    - apiVersion: v1
      kind: Secret
      metadata:
        name: copy-secret
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/source-secret"
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: copy-cm
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/source-cm"
`, ns.Name)

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

	// Verify the copied Secret has both fields.
	copySecret := &corev1.Secret{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "copy-secret", Namespace: ns.Name},
	}), copySecret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copySecret.Data).To(HaveKeyWithValue("field1", []byte("value1")))
	g.Expect(copySecret.Data).To(HaveKeyWithValue("field2", []byte("value2")))

	// Verify the copied ConfigMap has both fields.
	copyCM := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "copy-cm", Namespace: ns.Name},
	}), copyCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copyCM.Data).To(HaveKeyWithValue("field1", "value1"))
	g.Expect(copyCM.Data).To(HaveKeyWithValue("field2", "value2"))

	// Remove field2 from the source Secret.
	sourceSecret.Data = map[string][]byte{"field1": []byte("value1")}
	err = testClient.Update(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	// Remove field2 from the source ConfigMap.
	sourceCM.Data = map[string]string{"field1": "value1"}
	err = testClient.Update(ctx, sourceCM)
	g.Expect(err).ToNot(HaveOccurred())

	// Reconcile the ResourceSet after field removal.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Verify field2 was removed from the copied Secret.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(copySecret), copySecret)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copySecret.Data).To(HaveKeyWithValue("field1", []byte("value1")))
	g.Expect(copySecret.Data).NotTo(HaveKey("field2"))

	// Verify field2 was removed from the copied ConfigMap.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(copyCM), copyCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copyCM.Data).To(HaveKeyWithValue("field1", "value1"))
	g.Expect(copyCM.Data).NotTo(HaveKey("field2"))
}

func TestResourceSetReconciler_CopyFromBinaryData(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create source ConfigMap with both data and binaryData.
	sourceCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-cm-binary",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"config": "plain-text",
		},
		BinaryData: map[string][]byte{
			"cert.pem": []byte("binary-content"),
		},
	}
	err = testEnv.Create(ctx, sourceCM)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: test-binary-copy
  namespace: "%[1]s"
spec:
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: copy-cm-binary
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/copyFrom: "%[1]s/source-cm-binary"
`, ns.Name)

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

	// Check the ResourceSet was deployed.
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Verify the copied ConfigMap has both data and binaryData.
	copyCM := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "copy-cm-binary", Namespace: ns.Name},
	}), copyCM)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copyCM.Data).To(HaveKeyWithValue("config", "plain-text"))
	g.Expect(copyCM.BinaryData).To(HaveKeyWithValue("cert.pem", []byte("binary-content")))
}

func TestResourceSetReconciler_ChecksumFrom(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-config",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"greeting": "hello",
		},
	}
	err = testEnv.Create(ctx, cm)
	g.Expect(err).ToNot(HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-secret",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"token": []byte("s3cr3t"),
		},
	}
	err = testEnv.Create(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: podinfo
  namespace: "%[1]s"
spec:
  inputs:
    - name: podinfo
  resources:
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: << inputs.name >>
        namespace: "%[1]s"
      spec:
        selector:
          matchLabels:
            app.kubernetes.io/name: << inputs.name >>
        template:
          metadata:
            annotations:
              fluxcd.controlplane.io/checksumFrom: |
                ConfigMap/%[1]s/app-config,
                Secret/%[1]s/app-secret
            labels:
              app.kubernetes.io/name: << inputs.name >>
          spec:
            containers:
              - name: app
                image: ghcr.io/stefanprodan/podinfo:6.0.0
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize.
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// Verify the applied Deployment has both checksumFrom and checksum
	// annotations on its pod template.
	resultDep := &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "podinfo", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())

	// The annotation was written as a YAML '|' literal block scalar, so
	// its stored value contains newlines between the comma-separated refs
	// and a trailing newline. resolveChecksumFrom trims whitespace around
	// each ref, so the checksum is computed correctly regardless.
	templateAnnotations := resultDep.Spec.Template.Annotations
	g.Expect(templateAnnotations).To(HaveKeyWithValue(
		fluxcdv1.ChecksumFromAnnotation,
		fmt.Sprintf("ConfigMap/%[1]s/app-config,\nSecret/%[1]s/app-secret\n", ns.Name),
	))
	g.Expect(templateAnnotations).To(HaveKey(fluxcdv1.ChecksumAnnotation))
	firstChecksum := templateAnnotations[fluxcdv1.ChecksumAnnotation]
	g.Expect(firstChecksum).To(HavePrefix("sha256:"))

	// Idempotence: a second reconcile without source changes produces
	// the same checksum value.
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	resultDep = &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "podinfo", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(resultDep.Spec.Template.Annotations[fluxcdv1.ChecksumAnnotation]).To(Equal(firstChecksum))

	// Change the source ConfigMap data and expect the checksum to change.
	cm.Data["greeting"] = "hola"
	err = testClient.Update(ctx, cm)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	resultDep = &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "podinfo", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())
	secondChecksum := resultDep.Spec.Template.Annotations[fluxcdv1.ChecksumAnnotation]
	g.Expect(secondChecksum).To(HavePrefix("sha256:"))
	g.Expect(secondChecksum).ToNot(Equal(firstChecksum))

	// Change the source Secret data and expect the checksum to change again.
	secret.Data["token"] = []byte("new-s3cr3t")
	err = testClient.Update(ctx, secret)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	resultDep = &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "podinfo", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())
	thirdChecksum := resultDep.Spec.Template.Annotations[fluxcdv1.ChecksumAnnotation]
	g.Expect(thirdChecksum).To(HavePrefix("sha256:"))
	g.Expect(thirdChecksum).ToNot(Equal(secondChecksum))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ChecksumFromInSet(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// The ResourceSet renders both the ConfigMap and the Deployment.
	// The Deployment references the ConfigMap through checksumFrom, so
	// on the first reconcile there is no cluster copy to read from:
	// the checksum must be computed from the pending in-memory object.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: app
  namespace: "%[1]s"
spec:
  inputs:
    - greeting: hello
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: app-config
        namespace: "%[1]s"
      data:
        greeting: << inputs.greeting >>
    - apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: app
        namespace: "%[1]s"
      spec:
        selector:
          matchLabels:
            app.kubernetes.io/name: app
        template:
          metadata:
            annotations:
              fluxcd.controlplane.io/checksumFrom: "ConfigMap/%[1]s/app-config"
            labels:
              app.kubernetes.io/name: app
          spec:
            containers:
              - name: app
                image: ghcr.io/stefanprodan/podinfo:6.0.0
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.GetReason(result, meta.ReadyCondition)).To(BeIdenticalTo(meta.ReconciliationSucceededReason))

	// The checksum must be set even though the ConfigMap did not exist
	// in the cluster when reconciliation started.
	resultDep := &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "app", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())
	firstChecksum := resultDep.Spec.Template.Annotations[fluxcdv1.ChecksumAnnotation]
	g.Expect(firstChecksum).To(HavePrefix("sha256:"))

	// Change the inline ConfigMap data via the RS input and expect the
	// checksum to change — proving we hash rendered data, not stale
	// cluster data.
	obj = &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "app", Namespace: ns.Name}, obj)
	g.Expect(err).ToNot(HaveOccurred())
	obj.Spec.Inputs[0]["greeting"] = &apiextensionsv1.JSON{Raw: []byte(`"hola"`)}
	err = testClient.Update(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())

	resultDep = &appsv1.Deployment{}
	err = testClient.Get(ctx, client.ObjectKey{Name: "app", Namespace: ns.Name}, resultDep)
	g.Expect(err).ToNot(HaveOccurred())
	secondChecksum := resultDep.Spec.Template.Annotations[fluxcdv1.ChecksumAnnotation]
	g.Expect(secondChecksum).To(HavePrefix("sha256:"))
	g.Expect(secondChecksum).ToNot(Equal(firstChecksum))

	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ChecksumFromCanonicalParity(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// A cluster-stored ConfigMap with both data and binaryData.
	clusterCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parity-cm",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"greeting": "hello",
		},
		BinaryData: map[string][]byte{
			"blob": {0x00, 0x01, 0x02, 0x03},
		},
	}
	err = testEnv.Create(ctx, clusterCM)
	g.Expect(err).ToNot(HaveOccurred())

	// The equivalent rendered form: data copies through verbatim,
	// binaryData is base64-encoded in the unstructured representation.
	renderedCM := &unstructured.Unstructured{}
	renderedCM.SetAPIVersion("v1")
	renderedCM.SetKind("ConfigMap")
	renderedCM.SetName("parity-cm")
	renderedCM.SetNamespace(ns.Name)
	g.Expect(unstructured.SetNestedStringMap(renderedCM.Object, map[string]string{
		"greeting": "hello",
	}, "data")).To(Succeed())
	g.Expect(unstructured.SetNestedStringMap(renderedCM.Object, map[string]string{
		"blob": base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02, 0x03}),
	}, "binaryData")).To(Succeed())

	refs := fmt.Sprintf("ConfigMap/%s/parity-cm", ns.Name)

	clusterSum, err := newChecksumResolver(ctx, testClient, nil).resolve(refs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterSum).To(HavePrefix("sha256:"))

	// Empty refs from trailing, leading, or doubled commas are ignored.
	for _, variant := range []string{refs + ",", "," + refs, refs + ",,", ",," + refs + ",,"} {
		sum, err := newChecksumResolver(ctx, testClient, nil).resolve(variant)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sum).To(Equal(clusterSum))
	}

	inSetSum, err := newChecksumResolver(ctx, testClient,
		[]*unstructured.Unstructured{renderedCM}).resolve(refs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(inSetSum).To(Equal(clusterSum))

	// A Secret with both data (base64 in unstructured) and stringData.
	// When applied, the kube-apiserver merges stringData over data on
	// shared keys. The checksum must reflect that same precedence.
	clusterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parity-secret",
			Namespace: ns.Name,
		},
		Data: map[string][]byte{
			"shared":    []byte("from-stringData"),
			"data-only": {0xde, 0xad, 0xbe, 0xef},
		},
	}
	err = testEnv.Create(ctx, clusterSecret)
	g.Expect(err).ToNot(HaveOccurred())

	renderedSecret := &unstructured.Unstructured{}
	renderedSecret.SetAPIVersion("v1")
	renderedSecret.SetKind("Secret")
	renderedSecret.SetName("parity-secret")
	renderedSecret.SetNamespace(ns.Name)
	g.Expect(unstructured.SetNestedStringMap(renderedSecret.Object, map[string]string{
		"shared":    base64.StdEncoding.EncodeToString([]byte("from-data")),
		"data-only": base64.StdEncoding.EncodeToString([]byte{0xde, 0xad, 0xbe, 0xef}),
	}, "data")).To(Succeed())
	g.Expect(unstructured.SetNestedStringMap(renderedSecret.Object, map[string]string{
		"shared": "from-stringData",
	}, "stringData")).To(Succeed())

	secretRefs := fmt.Sprintf("Secret/%s/parity-secret", ns.Name)
	clusterSecretSum, err := newChecksumResolver(ctx, testClient, nil).resolve(secretRefs)
	g.Expect(err).ToNot(HaveOccurred())

	inSetSecretSum, err := newChecksumResolver(ctx, testClient,
		[]*unstructured.Unstructured{renderedSecret}).resolve(secretRefs)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(inSetSecretSum).To(Equal(clusterSecretSum))
}

func TestResourceSetReconciler_ChecksumFromErrors(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name        string
		checksumRef string
		wantErrMsg  string
	}{
		{
			name:        "missing source ConfigMap",
			checksumRef: fmt.Sprintf("ConfigMap/%s/does-not-exist", ns.Name),
			wantErrMsg:  "failed to resolve fluxcd.controlplane.io/checksumFrom reference ConfigMap",
		},
		{
			name:        "invalid reference format",
			checksumRef: "ConfigMap/missing-name",
			wantErrMsg:  "must be in the format 'Kind/namespace/name'",
		},
		{
			name:        "unsupported kind",
			checksumRef: fmt.Sprintf("Pod/%s/app", ns.Name),
			wantErrMsg:  "only ConfigMap and Secret are allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			name := fmt.Sprintf("err-%d", time.Now().UnixNano())
			objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: "%[2]s"
  namespace: "%[1]s"
spec:
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: "%[2]s"
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/checksumFrom: "%[3]s"
`, ns.Name, name, tc.checksumRef)

			obj := &fluxcdv1.ResourceSet{}
			err := yaml.Unmarshal([]byte(objDef), obj)
			g.Expect(err).ToNot(HaveOccurred())

			err = testEnv.Create(ctx, obj)
			g.Expect(err).ToNot(HaveOccurred())
			t.Cleanup(func() { _ = testClient.Delete(ctx, obj) })

			// Initialize.
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(obj),
			})
			g.Expect(err).ToNot(HaveOccurred())

			// Reconcile should fail with the expected error.
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(obj),
			})
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring(tc.wantErrMsg))
		})
	}
}

func TestResourceSetReconciler_ConvertKubeConfig(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret with kubeconfig data
	kubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUjRkdzMxSTVSK0F3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRFeU1qY3hOVEkyTWpoYUZ3MHpOREV5TWpVeE5UTXhNamhhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUM2dEhwVzEwcHlXU29ZSFpQdVFpVEY1bGh2SjB2RXJ0SWRzbWxpYXBpcHdRQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://test-cluster.example.com:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token-12345`

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"value": kubeconfigData,
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: prod
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig"
      data:
        cluster: << inputs.cluster >>
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
	g.Expect(result.Status.Inventory.Entries).To(HaveLen(1))
	g.Expect(result.Status.Inventory.Entries).To(ContainElements(
		fluxcdv1.ResourceRef{
			ID:      fmt.Sprintf("%s_prod-config__ConfigMap", ns.Name),
			Version: "v1",
		},
	))

	// Check if the ConfigMap was created with converted kubeconfig data.
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prod-config",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that the original data is preserved.
	g.Expect(resultCM.Data).To(HaveKeyWithValue("cluster", "prod"))

	// Verify that the kubeconfig fields were extracted and added.
	g.Expect(resultCM.Data).To(HaveKey("address"))
	g.Expect(resultCM.Data["address"]).To(Equal("https://test-cluster.example.com:6443"))
	g.Expect(resultCM.Data).To(HaveKey("ca.crt"))
	g.Expect(resultCM.Data["ca.crt"]).To(ContainSubstring("BEGIN CERTIFICATE"))
	g.Expect(resultCM.Data["ca.crt"]).To(ContainSubstring("END CERTIFICATE"))

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

	// Check if the ConfigMap was deleted.
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestResourceSetReconciler_ConvertKubeConfigWithExistingData(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret with kubeconfig data
	kubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUjRkdzMxSTVSK0F3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRFeU1qY3hOVEkyTWpoYUZ3MHpOREV5TWpVeE5UTXhNamhhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUM2dEhwVzEwcHlXU29ZSFpQdVFpVEY1bGh2SjB2RXJ0SWRzbWxpYXBpcHdRQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://test-cluster.example.com:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token-12345`

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"value": kubeconfigData,
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	// Test that existing server and ca.crt fields are NOT overwritten
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-preserve
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: staging
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-preserve
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig"
      data:
        cluster: << inputs.cluster >>
        address: https://existing-server.example.com:6443
        ca.crt: "existing-ca-cert-data"
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize and reconcile
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	// Check if the ConfigMap preserves existing data
	resultCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-config-preserve",
			Namespace: ns.Name,
		},
	}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(resultCM), resultCM)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify existing values were NOT overwritten
	g.Expect(resultCM.Data["address"]).To(Equal("https://existing-server.example.com:6443"))
	g.Expect(resultCM.Data["ca.crt"]).To(Equal("existing-ca-cert-data"))
	g.Expect(resultCM.Data["cluster"]).To(Equal("staging"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigInvalidAnnotation(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Test with invalid annotation format (missing namespace)
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-invalid
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: dev
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-invalid
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "invalid-format-without-slash"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile should fail with invalid annotation format
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("must be in the format 'namespace/name' or 'namespace/name:key'"))

	// Check the status
	result := &fluxcdv1.ResourceSet{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeFalse())

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigMissingSecret(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Test with non-existent source Secret
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-missing
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: qa
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-missing
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/non-existent-secret"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile should fail with missing Secret
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to get kubeconfig Secret"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigMissingValueField(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret without 'kubeconfig' or 'value' field
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig-novalue",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"other-key": "some-data",
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-novalue
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: test
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-novalue
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig-novalue"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile should fail with missing 'kubeconfig' or 'value' field
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not have 'kubeconfig' or 'value' field"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigFromKubeconfigKey(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret with kubeconfig data under the 'kubeconfig' key
	// (e.g. Crossplane Azure provider for AKS).
	kubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUjRkdzMxSTVSK0F3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRFeU1qY3hOVEkyTWpoYUZ3MHpOREV5TWpVeE5UTXhNamhhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUM2dEhwVzEwcHlXU29ZSFpQdVFpVEY1bGh2SjB2RXJ0SWRzbWxpYXBpcHdRQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://crossplane-aks.example.com:443
  name: crossplane-cluster
contexts:
- context:
    cluster: crossplane-cluster
    user: crossplane-user
  name: crossplane-context
current-context: crossplane-context
users:
- name: crossplane-user
  user:
    token: crossplane-token`

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig-kk",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"kubeconfig": kubeconfigData,
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-kk
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: aks
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-kk
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig-kk"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
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

	// Check the ConfigMap was created with converted kubeconfig data.
	resultCM := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "aks-config-kk", Namespace: ns.Name},
	}), resultCM)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(resultCM.Data).To(HaveKeyWithValue("cluster", "aks"))
	g.Expect(resultCM.Data["address"]).To(Equal("https://crossplane-aks.example.com:443"))
	g.Expect(resultCM.Data).To(HaveKey("ca.crt"))
	g.Expect(resultCM.Data["ca.crt"]).To(ContainSubstring("BEGIN CERTIFICATE"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigWithCustomKey(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret with kubeconfig data under a custom key.
	kubeconfigData := `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURCVENDQWUyZ0F3SUJBZ0lJUjRkdzMxSTVSK0F3RFFZSktvWklodmNOQVFFTEJRQXdGVEVUTUJFR0ExVUUKQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TkRFeU1qY3hOVEkyTWpoYUZ3MHpOREV5TWpVeE5UTXhNamhhTUJVeApFekFSQmdOVkJBTVRDbXQxWW1WeWJtVjBaWE13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLCkFvSUJBUUM2dEhwVzEwcHlXU29ZSFpQdVFpVEY1bGh2SjB2RXJ0SWRzbWxpYXBpcHdRQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    server: https://custom-key-cluster.example.com:6443
  name: custom-cluster
contexts:
- context:
    cluster: custom-cluster
    user: custom-user
  name: custom-context
current-context: custom-context
users:
- name: custom-user
  user:
    token: custom-token`

	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig-custom",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"my-kubeconfig": kubeconfigData,
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	// Use the 'namespace/name:key' format to specify the custom key.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-custom
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: custom
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-custom
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig-custom:my-kubeconfig"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
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

	// Check the ConfigMap was created with converted kubeconfig data.
	resultCM := &corev1.ConfigMap{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "custom-config-custom", Namespace: ns.Name},
	}), resultCM)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(resultCM.Data).To(HaveKeyWithValue("cluster", "custom"))
	g.Expect(resultCM.Data["address"]).To(Equal("https://custom-key-cluster.example.com:6443"))
	g.Expect(resultCM.Data).To(HaveKey("ca.crt"))
	g.Expect(resultCM.Data["ca.crt"]).To(ContainSubstring("BEGIN CERTIFICATE"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestResourceSetReconciler_ConvertKubeConfigCustomKeyMissing(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ns, err := testEnv.CreateNamespace(ctx, "test")
	g.Expect(err).ToNot(HaveOccurred())

	// Create a source Secret without the custom key.
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kubeconfig-custommissing",
			Namespace: ns.Name,
		},
		StringData: map[string]string{
			"value": "some-data",
		},
	}
	err = testEnv.Create(ctx, sourceSecret)
	g.Expect(err).ToNot(HaveOccurred())

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSet
metadata:
  name: kubeconfig-test-custommissing
  namespace: "%[1]s"
spec:
  inputs:
    - cluster: test
  resources:
    - apiVersion: v1
      kind: ConfigMap
      metadata:
        name: << inputs.cluster >>-config-custommissing
        namespace: "%[1]s"
        annotations:
          fluxcd.controlplane.io/convertKubeConfigFrom: "%[1]s/test-kubeconfig-custommissing:my-custom-key"
      data:
        cluster: << inputs.cluster >>
`, ns.Name)

	obj := &fluxcdv1.ResourceSet{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).ToNot(HaveOccurred())

	err = testEnv.Create(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())

	// Initialize
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Reconcile should fail with missing custom key
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not have 'my-custom-key' field"))

	// Cleanup
	err = testClient.Delete(ctx, obj)
	g.Expect(err).ToNot(HaveOccurred())
}
