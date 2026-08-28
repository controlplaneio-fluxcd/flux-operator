// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/cli-utils/pkg/object"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestDiffEngine(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	owner := createDiffOwner(t, "Kustomization", namespace, "apps", map[string]any{"prune": false})
	ownerRef := &DiffOwnerRef{Kind: "Kustomization", Namespace: namespace, Name: owner.GetName()}
	ownerLabels := map[string]string{
		"kustomize.toolkit.fluxcd.io/name":      owner.GetName(),
		"kustomize.toolkit.fluxcd.io/namespace": namespace,
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: create
  namespace: %s
data:
  value: one
`, namespace)
	output, err := testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/create create"))

	live := configMapUnstructured(namespace, "owned", ownerLabels, map[string]any{"keep": "one", "remove": "old"})
	g.Expect(applyDiffObject(ctx, live, "kustomize-controller")).To(Succeed())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: owned
  namespace: %s
data:
  keep: one
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/owned update"))
	g.Expect(output).To(ContainSubstring("path: /data/remove"))
	g.Expect(output).To(ContainSubstring("op: remove"))

	live = configMapUnstructured(namespace, "foreign", ownerLabels, map[string]any{"keep": "one", "foreign": "preserved"})
	g.Expect(applyDiffObject(ctx, live, "foreign-manager")).To(Succeed())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: foreign
  namespace: %s
data:
  keep: one
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("No changes detected"))
	g.Expect(output).NotTo(ContainSubstring("path: /data/foreign"))

	metadataOnly := configMapUnstructured(namespace, "metadata-only", ownerLabels, map[string]any{"keep": "one"})
	metadataOnly.SetAnnotations(map[string]string{"removed-by-apply": "old"})
	g.Expect(applyDiffObject(ctx, metadataOnly, "kustomize-controller")).To(Succeed())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: metadata-only
  namespace: %s
data:
  keep: one
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("No changes detected"))
	g.Expect(output).NotTo(ContainSubstring("ConfigMap/" + namespace + "/metadata-only update"))

	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: owned
  namespace: %s
  annotations:
    kustomize.toolkit.fluxcd.io/ssa: ignore
data:
  keep: two
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("skipped (kustomize.toolkit.fluxcd.io/ssa: ignore)"))

	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: owned
  namespace: %s
  annotations:
    kustomize.toolkit.fluxcd.io/ssa: IfNotPresent
data:
  keep: changed
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("skipped (kustomize.toolkit.fluxcd.io/ssa: IfNotPresent)"))

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "token", Namespace: namespace, Labels: ownerLabels}, Data: map[string][]byte{"token": []byte("before")}}
	g.Expect(testClient.Client.Create(ctx, secret)).To(Succeed())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: token
  namespace: %s
stringData:
  token: after
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Secret/" + namespace + "/token update"))
	g.Expect(output).To(ContainSubstring("*** (after)"))
	g.Expect(output).NotTo(ContainSubstring("before"))

	manifest = fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: invalid
  namespace: %s
data:
  token: 'TOP-SECRET-%%%%%%'
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Secret/" + namespace + "/invalid error:"))
	g.Expect(output).NotTo(ContainSubstring("TOP-SECRET"))

	immutable := true
	immutableCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "immutable", Namespace: namespace, Labels: ownerLabels},
		Immutable:  &immutable,
		Data:       map[string]string{"value": "before"},
	}
	g.Expect(testClient.Client.Create(ctx, immutableCM)).To(Succeed())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: immutable
  namespace: %s
immutable: true
data:
  value: after
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/immutable error:"))

	forcedOwner := owner.DeepCopy()
	forcedOwner.Object["spec"].(map[string]any)["force"] = true
	forcedYAML := mustObjectsYAML(t, forcedOwner)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, FluxObject: forcedYAML})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/immutable recreate (forced, not validated)"))
}

func TestDiffHelmReleasePolish(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	owner := createDiffOwner(t, "HelmRelease", namespace, "app", map[string]any{})
	ownerRef := &DiffOwnerRef{Kind: "HelmRelease", Namespace: namespace, Name: owner.GetName()}
	labels := map[string]string{
		"app.kubernetes.io/managed-by":     "Helm",
		"helm.sh/chart":                    "app-1.0.0_deadbeef",
		"helm.toolkit.fluxcd.io/name":      owner.GetName(),
		"helm.toolkit.fluxcd.io/namespace": namespace,
	}
	live := configMapUnstructured(namespace, "app", labels, map[string]any{"value": "live"})
	live.SetAnnotations(map[string]string{
		"meta.helm.sh/release-name":      owner.GetName(),
		"meta.helm.sh/release-namespace": namespace,
	})
	g.Expect(applyDiffObject(ctx, live, "helm-controller")).To(Succeed())
	setOwnerInventory(t, owner, live)

	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: app
  namespace: %s
  labels:
    helm.sh/chart: app-v1.0.0
data:
  value: live
---
apiVersion: hooks.example.com/v1
kind: StartupCheck
metadata:
  name: startup
  namespace: %s
  annotations:
    helm.sh/hook: post-install
`, namespace, namespace)
	output, err := testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("StartupCheck/" + namespace + "/startup skipped (helm.sh/hook: post-install)"))
	g.Expect(output).To(ContainSubstring("Summary: 1 unchanged, 1 skipped"))
	g.Expect(output).NotTo(ContainSubstring("app-1.0.0_deadbeef"))
	g.Expect(output).NotTo(ContainSubstring("app-v1.0.0"))

	manifest = strings.Replace(manifest, "value: live", "value: desired", 1)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/app update"))
	g.Expect(output).To(ContainSubstring("path: /data/value"))
	g.Expect(output).NotTo(ContainSubstring("helm.sh/chart"))
	g.Expect(output).NotTo(ContainSubstring("app-1.0.0_deadbeef"))
	g.Expect(output).NotTo(ContainSubstring("app-v1.0.0"))
}

func TestDiffDependentResourcesAndOwnership(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	missingNamespace := fmt.Sprintf("diff-missing-%d", time.Now().UnixNano())
	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: app-one
  namespace: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-two
  namespace: %s
`, missingNamespace, missingNamespace)
	output, err := testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	warning := fmt.Sprintf("namespace %s does not exist in the cluster, objects in it are not validated", missingNamespace)
	g.Expect(strings.Count(output, warning)).To(Equal(1))
	g.Expect(output).To(ContainSubstring("ConfigMap/" + missingNamespace + "/app-one create (not validated: namespace " + missingNamespace + " does not exist yet)"))
	g.Expect(output).To(ContainSubstring("ConfigMap/" + missingNamespace + "/app-two create (not validated: namespace " + missingNamespace + " does not exist yet)"))
	g.Expect(output).NotTo(ContainSubstring(" error:"))

	inManifestNamespace := fmt.Sprintf("diff-in-manifest-%d", time.Now().UnixNano())
	manifest = fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app
  namespace: %s
`, inManifestNamespace, inManifestNamespace)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("namespace " + inManifestNamespace + " does not exist in the cluster, objects in it are not validated"))
	g.Expect(output).To(ContainSubstring("ConfigMap/" + inManifestNamespace + "/app create (not validated: namespace " + inManifestNamespace + " does not exist yet)"))

	widgetManifest := func(group string, includeCRD bool) string {
		resources := fmt.Sprintf(`apiVersion: %s/v1
kind: Widget
metadata:
  name: app-one
---
apiVersion: %s/v1
kind: Widget
metadata:
  name: app-two
`, group, group)
		if !includeCRD {
			return resources
		}
		return fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.%s
spec:
  group: %s
  names:
    kind: Widget
    plural: widgets
    singular: widget
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
---
%s`, group, group, resources)
	}

	group := fmt.Sprintf("diff%d.example.com", time.Now().UnixNano())
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: widgetManifest(group, false)})
	g.Expect(err).NotTo(HaveOccurred())
	warning = fmt.Sprintf("CRD for %s/Widget does not exist in the cluster, objects of that kind are not validated", group)
	g.Expect(strings.Count(output, warning)).To(Equal(1))
	g.Expect(output).To(ContainSubstring("Widget/app-one create (not validated: CRD for " + group + "/Widget does not exist yet)"))
	g.Expect(output).To(ContainSubstring("Widget/app-two create (not validated: CRD for " + group + "/Widget does not exist yet)"))
	g.Expect(output).NotTo(ContainSubstring(" error:"))

	inManifestGroup := fmt.Sprintf("diff%d.example.com", time.Now().UnixNano()+1)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: widgetManifest(inManifestGroup, true)})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("CRD for " + inManifestGroup + "/Widget does not exist in the cluster, objects of that kind are not validated"))
	g.Expect(output).To(ContainSubstring("Widget/app-one create (not validated: CRD for " + inManifestGroup + "/Widget does not exist yet)"))

	namespace := createDiffNamespace(t)
	owner := createDiffOwner(t, "Kustomization", namespace, "suspended", map[string]any{"suspend": true, "prune": false})
	ownerRef := &DiffOwnerRef{Kind: "Kustomization", Namespace: namespace, Name: owner.GetName()}
	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: hinted
  namespace: %s
  labels:
    kustomize.toolkit.fluxcd.io/name: other
    kustomize.toolkit.fluxcd.io/namespace: %s
data: {value: live}
`, namespace, namespace)
	objects, err := readTestObjects(manifest)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(applyDiffObject(ctx, objects[0], "other-manager")).To(Succeed())
	manifest = strings.Replace(manifest, "value: live", "value: desired", 1)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest, OwnerRef: ownerRef})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("suspended: changes are not applied until resumed"))
	g.Expect(output).To(ContainSubstring("currently managed by Kustomization/" + namespace + "/other"))

	future := ownerObject("HelmRelease", "future", namespace, false)
	output, err = testClient.Diff(ctx, DiffRequest{
		Manifest:   fmt.Sprintf("apiVersion: v1\nkind: ConfigMap\nmetadata: {name: precedence, namespace: %s}\n", namespace),
		FluxObject: mustObjectsYAML(t, future),
		OwnerRef:   ownerRef,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Diff for HelmRelease/" + namespace + "/future"))

	output, err = testClient.Diff(ctx, DiffRequest{Manifest: fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  generateName: generated-
  namespace: %s
`, namespace)})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("error: metadata.generateName is not supported without metadata.name"))

	user, err := testEnv.AddUser(envtest.User{Name: "diff-no-rbac"}, nil)
	g.Expect(err).NotTo(HaveOccurred())
	restrictedConfig := user.Config()
	httpClient, err := rest.HTTPClientFor(restrictedConfig)
	g.Expect(err).NotTo(HaveOccurred())
	mapper, err := apiutil.NewDynamicRESTMapper(restrictedConfig, httpClient)
	g.Expect(err).NotTo(HaveOccurred())
	restrictedClient, err := ctrlclient.New(restrictedConfig, ctrlclient.Options{Scheme: NewTestScheme(), Mapper: mapper})
	g.Expect(err).NotTo(HaveOccurred())
	output, err = NewClient(restrictedClient, restrictedConfig, mapper).Diff(ctx, DiffRequest{Manifest: fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: forbidden
  namespace: %s
`, namespace)})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("error: dry-run requires get and patch permission on ConfigMap/" + namespace + "/forbidden"))
}

func TestDiffPruneFiltering(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	owner := createDiffOwner(t, "Kustomization", namespace, "pruner", map[string]any{"prune": true})
	ownerLabels := map[string]string{
		"kustomize.toolkit.fluxcd.io/name":      owner.GetName(),
		"kustomize.toolkit.fluxcd.io/namespace": namespace,
	}

	deleteObject := configMapUnstructured(namespace, "delete-me", ownerLabels, map[string]any{"value": "old"})
	mismatch := configMapUnstructured(namespace, "mismatch", map[string]string{
		"kustomize.toolkit.fluxcd.io/name": "other", "kustomize.toolkit.fluxcd.io/namespace": namespace,
	}, map[string]any{"value": "old"})
	disabled := configMapUnstructured(namespace, "disabled", ownerLabels, map[string]any{"value": "old"})
	disabled.SetAnnotations(map[string]string{"kustomize.toolkit.fluxcd.io/prune": "disabled"})
	for _, item := range []*unstructured.Unstructured{deleteObject, mismatch, disabled} {
		g.Expect(testClient.Client.Create(ctx, item)).To(Succeed())
	}
	missing := configMapUnstructured(namespace, "missing", ownerLabels, nil)
	setOwnerInventory(t, owner, deleteObject, mismatch, disabled, missing)

	missingPruneNamespace := fmt.Sprintf("diff-prune-missing-%d", time.Now().UnixNano())
	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: current
  namespace: %s
data: {value: current}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: unvalidated
  namespace: %s
`, namespace, missingPruneNamespace)
	output, err := testClient.Diff(ctx, DiffRequest{
		Manifest: manifest,
		OwnerRef: &DiffOwnerRef{Kind: "Kustomization", Namespace: namespace, Name: owner.GetName()},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + missingPruneNamespace + "/unvalidated create (not validated: namespace " + missingPruneNamespace + " does not exist yet)"))
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/delete-me delete"))
	g.Expect(output).NotTo(ContainSubstring("mismatch delete"))
	g.Expect(output).NotTo(ContainSubstring("disabled delete"))
	g.Expect(output).NotTo(ContainSubstring("missing delete"))

	helmOwner := createDiffOwner(t, "HelmRelease", namespace, "helm-pruner", map[string]any{})
	helmLabels := map[string]string{
		"helm.toolkit.fluxcd.io/name":      helmOwner.GetName(),
		"helm.toolkit.fluxcd.io/namespace": namespace,
	}
	helmKeep := configMapUnstructured(namespace, "helm-keep", helmLabels, map[string]any{"value": "old"})
	helmKeep.SetAnnotations(map[string]string{"helm.sh/resource-policy": "keep"})
	g.Expect(testClient.Client.Create(ctx, helmKeep)).To(Succeed())
	crdGroup := fmt.Sprintf("prune%d.example.com", time.Now().UnixNano())
	crdObjects, err := readTestObjects(fmt.Sprintf(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: kepts.%s
  labels:
    helm.toolkit.fluxcd.io/name: helm-pruner
    helm.toolkit.fluxcd.io/namespace: %s
spec:
  group: %s
  names:
    kind: Kept
    plural: kepts
    singular: kept
  scope: Cluster
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
`, crdGroup, namespace, crdGroup))
	g.Expect(err).NotTo(HaveOccurred())
	helmCRD := crdObjects[0]
	g.Expect(testClient.Client.Create(ctx, helmCRD)).To(Succeed())
	setOwnerInventory(t, helmOwner, helmKeep, helmCRD)

	manifest = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: helm-current
  namespace: %s
data: {value: current}
`, namespace)
	output, err = testClient.Diff(ctx, DiffRequest{
		Manifest: manifest,
		OwnerRef: &DiffOwnerRef{Kind: "HelmRelease", Namespace: namespace, Name: helmOwner.GetName()},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).NotTo(ContainSubstring("helm-keep delete"))
	g.Expect(output).NotTo(ContainSubstring("CustomResourceDefinition/" + helmCRD.GetName() + " delete"))
}

func createDiffNamespace(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "diff-"}}
	g.Expect(testClient.Client.Create(context.Background(), namespace)).To(Succeed())
	return namespace.Name
}

func createDiffOwner(t *testing.T, kind, namespace, name string, spec map[string]any) *unstructured.Unstructured {
	t.Helper()
	g := NewWithT(t)
	owner := ownerObject(kind, name, namespace, false)
	for key, value := range spec {
		owner.Object["spec"].(map[string]any)[key] = value
	}
	g.Expect(testClient.Client.Create(context.Background(), owner)).To(Succeed())
	return owner
}

func configMapUnstructured(namespace, name string, labels map[string]string, data map[string]any) *unstructured.Unstructured {
	resource := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}}
	resource.SetLabels(labels)
	if data != nil {
		resource.Object["data"] = data
	}
	return resource
}

func applyDiffObject(ctx context.Context, resource *unstructured.Unstructured, manager string) error {
	return testClient.Client.Patch(ctx, resource, ctrlclient.Apply, ctrlclient.FieldOwner(manager), ctrlclient.ForceOwnership)
}

func mustObjectsYAML(t *testing.T, objects ...*unstructured.Unstructured) string {
	t.Helper()
	result, err := ssautil.ObjectsToYAML(objects)
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return result
}

func readTestObjects(manifest string) ([]*unstructured.Unstructured, error) {
	return ssautil.ReadObjects(strings.NewReader(manifest))
}

func setOwnerInventory(t *testing.T, owner *unstructured.Unstructured, objects ...*unstructured.Unstructured) {
	t.Helper()
	g := NewWithT(t)
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(owner.GroupVersionKind())
	g.Expect(testClient.Client.Get(context.Background(), ctrlclient.ObjectKeyFromObject(owner), current)).To(Succeed())
	entries := make([]any, 0, len(objects))
	for _, item := range objects {
		entries = append(entries, map[string]any{
			"id": object.UnstructuredToObjMetadata(item).String(), "v": item.GroupVersionKind().Version,
		})
	}
	g.Expect(unstructured.SetNestedSlice(current.Object, entries, "status", "inventory", "entries")).To(Succeed())
	g.Expect(testClient.Client.Status().Update(context.Background(), current)).To(Succeed())
}

func TestDiffMonorepoConformance(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "apps-staging"}}
	g.Expect(testClient.Client.Create(ctx, namespace)).To(Succeed())
	vars := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "flux-vars", Namespace: namespace.Name},
		Data: map[string]string{
			"env": "dev", "cluster_registry": "flux-registry:5000", "app_registry": "flux-registry:5000",
			"app_registry_insecure": "true", "cluster_name": "kind-flux",
		},
	}
	g.Expect(testClient.Client.Create(ctx, vars)).To(Succeed())

	fluxBuild := readFixture(t, "testdata/monorepo/backend-staging-fluxbuild.yaml")
	liveObjects, err := readTestObjects(fluxBuild)
	g.Expect(err).NotTo(HaveOccurred())
	for _, resource := range liveObjects {
		g.Expect(applyDiffObject(ctx, resource, "kustomize-controller")).To(Succeed())
	}

	ownerObjects, err := readTestObjects(readFixture(t, "testdata/monorepo/ks-backend-staging-live.yaml"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ownerObjects).To(HaveLen(1))
	owner := ownerObjects[0]
	owner.SetCreationTimestamp(metav1.Time{})
	owner.SetFinalizers(nil)
	owner.SetGeneration(0)
	owner.SetResourceVersion("")
	owner.SetUID("")
	g.Expect(testClient.Client.Create(ctx, owner)).To(Succeed())
	setOwnerInventory(t, owner, liveObjects...)

	request := DiffRequest{
		Manifest: readFixture(t, "testdata/monorepo/backend-staging-build.yaml"),
		OwnerRef: &DiffOwnerRef{Kind: "Kustomization", Namespace: namespace.Name, Name: "backend"},
	}
	output, err := testClient.Diff(ctx, request)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(Equal("Diff for Kustomization/apps-staging/backend (field manager: kustomize-controller, prune: enabled)\n\nNo changes detected\n"))

	// This represents the raw output after rebuilding the overlay with one
	// ConfigMap value changed; Kustomize updates both the hash and its reference.
	request.Manifest = strings.Replace(request.Manifest, "PODINFO_LEVEL: info", "PODINFO_LEVEL: debug", 1)
	request.Manifest = strings.ReplaceAll(request.Manifest, "backend-m759gh88kd", "backend-6d225hh94m")
	output, err = testClient.Diff(ctx, request)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/apps-staging/backend-6d225hh94m create"))
	g.Expect(output).To(ContainSubstring("Deployment/apps-staging/backend update"))
	g.Expect(output).To(ContainSubstring("path: /spec/template/spec/containers/0/envFrom/0/configMapRef/name"))
	g.Expect(output).To(ContainSubstring("value: backend-6d225hh94m"))
	g.Expect(strings.Count(output, "- op:")).To(Equal(1))
	g.Expect(output).To(ContainSubstring("ConfigMap/apps-staging/backend-m759gh88kd delete"))
	g.Expect(output).To(ContainSubstring("Summary: 1 create, 1 update, 2 unchanged, 1 delete"))
}

func TestDiffSOPSKeyComparison(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	live := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "encrypted", Namespace: namespace},
		Data:       map[string][]byte{"token": []byte("cleartext")},
	}
	g.Expect(testClient.Client.Create(ctx, live)).To(Succeed())

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: encrypted
  namespace: %s
stringData:
  token: ENC[AES256_GCM,data:ciphertext]
sops:
  mac: ENC[AES256_GCM,data:secret-ciphertext]
`, namespace)
	output, err := testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("No changes detected"))
	g.Expect(output).NotTo(ContainSubstring("ciphertext"))

	manifest = strings.Replace(manifest, "  token: ENC", "  added: ENC[AES256_GCM,data:other]\n  token: ENC", 1)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Secret/" + namespace + "/encrypted update (sops encrypted: keys compared only)"))
	g.Expect(output).NotTo(ContainSubstring("ciphertext"))

	manifest = strings.Replace(manifest, "name: encrypted", "name: encrypted-new", 1)
	output, err = testClient.Diff(ctx, DiffRequest{Manifest: manifest})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Secret/" + namespace + "/encrypted-new create"))
	g.Expect(output).NotTo(ContainSubstring("ciphertext"))
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return string(data)
}
