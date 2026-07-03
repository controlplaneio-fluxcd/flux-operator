// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package controller

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/conditions"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inputs"
)

var (
	registerCRDOnce sync.Once
)

func ensureExternalArtifactCRD(ctx context.Context, t *testing.T) {
	t.Helper()
	registerCRDOnce.Do(func() {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "externalartifacts.source.toolkit.fluxcd.io",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "source.toolkit.fluxcd.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     "ExternalArtifact",
					ListKind: "ExternalArtifactList",
					Plural:   "externalartifacts",
					Singular: "externalartifact",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
									},
									"status": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"artifact": {
												Type: "object",
												Properties: map[string]apiextensionsv1.JSONSchemaProps{
													"revision": {
														Type: "string",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		err := testClient.Create(ctx, crd)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			t.Fatalf("failed to create ExternalArtifact CRD: %v", err)
		}
	})
}

func newExternalArtifact(name, namespace string, labels map[string]string, revision string) *unstructured.Unstructured {
	ea := &unstructured.Unstructured{}
	ea.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "ExternalArtifact",
	})
	ea.SetName(name)
	ea.SetNamespace(namespace)
	ea.SetLabels(labels)
	if revision != "" {
		_ = unstructured.SetNestedField(ea.Object, revision, "status", "artifact", "revision")
	}
	return ea
}

func TestResourceSetInputProviderReconciler_ExternalArtifact_CELValidation(t *testing.T) {
	g := NewWithT(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ensureExternalArtifactCRD(ctx, t)

	ns, err := testEnv.CreateNamespace(ctx, "test-ea-cel")
	g.Expect(err).ToNot(HaveOccurred())

	// ExternalArtifact without selector must be rejected.
	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea-no-selector
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.selector must be set when spec.type is 'ExternalArtifact'"))

	// Non-ExternalArtifact type with selector must be rejected.
	objDefWithSel := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-static-with-selector
  namespace: "%[1]s"
spec:
  type: Static
  selector:
    matchLabels:
      env: dev
`, ns.Name)
	objWithSel := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDefWithSel), objWithSel)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, objWithSel)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.selector must not be set when spec.type is not 'ExternalArtifact'"))

	// ExternalArtifact with selector must be accepted.
	objDefOK := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea-valid
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
  selector:
    matchLabels:
      env: dev
`, ns.Name)
	objOK := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDefOK), objOK)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, objOK)
	g.Expect(err).NotTo(HaveOccurred())

	// ExternalArtifact with url must be rejected.
	objDefWithURL := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea-with-url
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
  url: https://example.com
  selector:
    matchLabels:
      env: dev
`, ns.Name)
	objWithURL := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDefWithURL), objWithURL)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, objWithURL)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("spec.url must be empty when spec.type is 'ExternalArtifact'"))

	// ExternalArtifact with secretRef must be rejected.
	objDefWithSecret := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea-with-secret
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
  selector:
    matchLabels:
      env: dev
  secretRef:
    name: my-secret
`, ns.Name)
	objWithSecret := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDefWithSecret), objWithSecret)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, objWithSecret)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot specify spec.secretRef"))

	// ExternalArtifact with certSecretRef must be rejected.
	objDefWithCert := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea-with-cert
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
  selector:
    matchLabels:
      env: dev
  certSecretRef:
    name: my-cert
`, ns.Name)
	objWithCert := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDefWithCert), objWithCert)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, objWithCert)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cannot specify spec.certSecretRef"))
}

func TestResourceSetInputProviderReconciler_ExternalArtifact_RuntimeGuard(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Reconcile with nil selector should stall.
	obj := &fluxcdv1.ResourceSetInputProvider{
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalArtifact,
		},
	}

	r, err := reconciler.reconcile(ctx, obj, nil)
	g.Expect(r).To(Equal(reconcile.Result{}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsStalled(obj)).To(BeTrue())
	g.Expect(conditions.GetReason(obj, meta.StalledCondition)).To(Equal(fluxcdv1.ReasonInvalidSpec))
	g.Expect(conditions.GetMessage(obj, meta.StalledCondition)).To(ContainSubstring("spec.selector must be set"))
}

func TestResourceSetInputProviderReconciler_ExternalArtifact_Reconcile(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ensureExternalArtifactCRD(ctx, t)

	ns, err := testEnv.CreateNamespace(ctx, "test-ea-reconcile")
	g.Expect(err).ToNot(HaveOccurred())

	// Create ExternalArtifact objects in the namespace.
	artifacts := []*unstructured.Unstructured{
		newExternalArtifact("dev-apps-auth", ns.Name, map[string]string{"env": "dev", "app": "auth"}, "sha256:abc123"),
		newExternalArtifact("dev-apps-payments", ns.Name, map[string]string{"env": "dev", "app": "payments"}, "sha256:def456"),
		// Different env — should NOT match.
		newExternalArtifact("prod-apps-auth", ns.Name, map[string]string{"env": "prod", "app": "auth"}, "sha256:xyz789"),
	}
	for _, ea := range artifacts {
		g.Expect(testClient.Create(ctx, ea)).To(Succeed())
	}

	objDef := fmt.Sprintf(`
apiVersion: fluxcd.controlplane.io/v1
kind: ResourceSetInputProvider
metadata:
  name: test-ea
  namespace: "%[1]s"
spec:
  type: ExternalArtifact
  selector:
    matchLabels:
      env: dev
`, ns.Name)

	obj := &fluxcdv1.ResourceSetInputProvider{}
	err = yaml.Unmarshal([]byte(objDef), obj)
	g.Expect(err).NotTo(HaveOccurred())
	err = testEnv.Create(ctx, obj)
	g.Expect(err).NotTo(HaveOccurred())

	// Initialize (adds finalizer).
	r, err := reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeTrue())

	// Second reconcile: should produce exported inputs.
	r, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(r.Requeue).To(BeFalse())

	result := &fluxcdv1.ResourceSetInputProvider{}
	err = testClient.Get(ctx, client.ObjectKeyFromObject(obj), result)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(conditions.IsReady(result)).To(BeTrue())

	// Should have exactly 2 inputs (auth and payments; prod excluded).
	g.Expect(result.Status.ExportedInputs).To(HaveLen(2))

	// Build a name→inputMap index for assertions.
	exportedMap := make(map[string]map[string]any, 2)
	for _, ei := range result.Status.ExportedInputs {
		raw, merr := yaml.Marshal(ei)
		g.Expect(merr).NotTo(HaveOccurred())
		var parsed map[string]any
		g.Expect(yaml.Unmarshal(raw, &parsed)).To(Succeed())
		nameVal, ok := parsed["name"]
		g.Expect(ok).To(BeTrue(), "exported input missing 'name' field")
		exportedMap[fmt.Sprintf("%v", nameVal)] = parsed
	}

	for _, artifactName := range []string{"dev-apps-auth", "dev-apps-payments"} {
		input, ok := exportedMap[artifactName]
		g.Expect(ok).To(BeTrue(), "missing exported input for artifact %s", artifactName)
		g.Expect(input["namespace"]).To(Equal(ns.Name))
		g.Expect(input["env"]).To(Equal("dev"))
		g.Expect(input).To(HaveKey("revision"))
		g.Expect(input["id"]).To(Equal(inputs.ID(fmt.Sprintf("%s+%s", ns.Name, artifactName))))
	}
	g.Expect(exportedMap["dev-apps-auth"]["app"]).To(Equal("auth"))
	g.Expect(exportedMap["dev-apps-payments"]["app"]).To(Equal("payments"))

	// Revision should be stable on a second reconcile.
	lastRevision := result.Status.LastExportedRevision
	_, err = reconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(obj),
	})
	g.Expect(err).NotTo(HaveOccurred())

	result2 := &fluxcdv1.ResourceSetInputProvider{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(obj), result2)).To(Succeed())
	g.Expect(result2.Status.LastExportedRevision).To(Equal(lastRevision))
}

func TestResourceSetInputProviderReconciler_ExternalArtifact_EdgeCases(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ensureExternalArtifactCRD(ctx, t)

	ns, err := testEnv.CreateNamespace(ctx, "test-ea-edgecases")
	g.Expect(err).ToNot(HaveOccurred())

	// 1. Create ExternalArtifact objects in the namespace.
	artifacts := []*unstructured.Unstructured{
		newExternalArtifact("ea-1", ns.Name, map[string]string{"env": "dev", "app": "one"}, "sha256:111"),
		newExternalArtifact("ea-2", ns.Name, map[string]string{"env": "dev", "app": "two"}, "sha256:222"),
		// ea-3 has no revision
		newExternalArtifact("ea-3", ns.Name, map[string]string{"env": "dev", "app": "three"}, ""),
	}
	for _, ea := range artifacts {
		g.Expect(testClient.Create(ctx, ea)).To(Succeed())
	}

	// Case A: Limit filter set to 2.
	objLimit := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ea-limit",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalArtifact,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "dev"},
			},
			Filter: &fluxcdv1.ResourceSetInputFilter{
				Limit: 2,
			},
		},
	}
	g.Expect(testEnv.Create(ctx, objLimit)).To(Succeed())

	// Reconcile and check limit
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objLimit)})
	g.Expect(err).NotTo(HaveOccurred())
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objLimit)})
	g.Expect(err).NotTo(HaveOccurred())

	resultLimit := &fluxcdv1.ResourceSetInputProvider{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(objLimit), resultLimit)).To(Succeed())
	g.Expect(conditions.IsReady(resultLimit)).To(BeTrue())
	g.Expect(resultLimit.Status.ExportedInputs).To(HaveLen(2))

	// Case B: Check that "ea-3" (no revision) is successfully resolved and has no "revision" key in exported map.
	// We check this by using a provider without limit.
	objNoLimit := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ea-nolimit",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalArtifact,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "dev"},
			},
		},
	}
	g.Expect(testEnv.Create(ctx, objNoLimit)).To(Succeed())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objNoLimit)})
	g.Expect(err).NotTo(HaveOccurred())
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objNoLimit)})
	g.Expect(err).NotTo(HaveOccurred())

	resultNoLimit := &fluxcdv1.ResourceSetInputProvider{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(objNoLimit), resultNoLimit)).To(Succeed())
	g.Expect(conditions.IsReady(resultNoLimit)).To(BeTrue())
	g.Expect(resultNoLimit.Status.ExportedInputs).To(HaveLen(3))

	foundThree := false
	for _, ei := range resultNoLimit.Status.ExportedInputs {
		raw, merr := yaml.Marshal(ei)
		g.Expect(merr).NotTo(HaveOccurred())
		var parsed map[string]any
		g.Expect(yaml.Unmarshal(raw, &parsed)).To(Succeed())
		if parsed["name"] == "ea-3" {
			foundThree = true
			g.Expect(parsed).NotTo(HaveKey("revision"))
		} else {
			g.Expect(parsed).To(HaveKey("revision"))
		}
	}
	g.Expect(foundThree).To(BeTrue())

	// Case C: Empty list (no matching artifacts).
	objEmpty := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ea-empty",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalArtifact,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "non-existent"},
			},
		},
	}
	g.Expect(testEnv.Create(ctx, objEmpty)).To(Succeed())

	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objEmpty)})
	g.Expect(err).NotTo(HaveOccurred())
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(objEmpty)})
	g.Expect(err).NotTo(HaveOccurred())

	resultEmpty := &fluxcdv1.ResourceSetInputProvider{}
	g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(objEmpty), resultEmpty)).To(Succeed())
	g.Expect(conditions.IsReady(resultEmpty)).To(BeTrue())
	g.Expect(resultEmpty.Status.ExportedInputs).To(BeEmpty())
}

func TestResourceSetInputProviderReconciler_ExternalArtifact_Watch(t *testing.T) {
	g := NewWithT(t)
	reconciler := getResourceSetInputProviderReconciler(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ensureExternalArtifactCRD(ctx, t)

	ns, err := testEnv.CreateNamespace(ctx, "test-ea-watch")
	g.Expect(err).ToNot(HaveOccurred())

	// 1. Create a ResourceSetInputProvider.
	rsip := &fluxcdv1.ResourceSetInputProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watcher-rsip",
			Namespace: ns.Name,
		},
		Spec: fluxcdv1.ResourceSetInputProviderSpec{
			Type: fluxcdv1.InputProviderExternalArtifact,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "dev", "team": "platform"},
			},
		},
	}
	g.Expect(testEnv.Create(ctx, rsip)).To(Succeed())

	// 2. Create a matching ExternalArtifact.
	matchingArtifact := newExternalArtifact("match", ns.Name, map[string]string{
		"env":  "dev",
		"team": "platform",
		"app":  "auth",
	}, "sha256:111")

	// 3. Create a non-matching ExternalArtifact.
	nonMatchingArtifact := newExternalArtifact("no-match", ns.Name, map[string]string{
		"env":  "prod",
		"team": "platform",
		"app":  "auth",
	}, "sha256:222")

	// 4. Verify map function returns requests.
	reqsMatching := reconciler.requestsForExternalArtifacts(ctx, matchingArtifact)
	g.Expect(reqsMatching).To(HaveLen(1))
	g.Expect(reqsMatching[0].Name).To(Equal("watcher-rsip"))
	g.Expect(reqsMatching[0].Namespace).To(Equal(ns.Name))

	reqsNonMatching := reconciler.requestsForExternalArtifacts(ctx, nonMatchingArtifact)
	g.Expect(reqsNonMatching).To(BeEmpty())
}
