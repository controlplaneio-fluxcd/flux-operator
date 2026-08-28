// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"testing"

	"github.com/fluxcd/pkg/ssa"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPatch(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	configMapGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "merge", Namespace: namespace},
		Data:       map[string]string{"value": "before"},
	}
	g.Expect(testClient.Client.Create(ctx, cm)).To(Succeed())
	output, err := testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: cm.Name, Namespace: namespace, Patch: "data:\n  value: after\n",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/merge patched"))
	g.Expect(output).To(ContainSubstring("op: replace"))
	g.Expect(output).To(ContainSubstring("path: /data/value"))
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(cm), cm)).To(Succeed())
	g.Expect(cm.Data["value"]).To(Equal("after"))

	finalized := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "finalized", Namespace: namespace, Finalizers: []string{"example.com/one"}},
	}
	g.Expect(testClient.Client.Create(ctx, finalized)).To(Succeed())
	output, err = testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: finalized.Name, Namespace: namespace,
		Type: "json", Patch: `[{"op":"remove","path":"/metadata/finalizers/0"}]`,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("op: remove"))
	g.Expect(output).To(ContainSubstring("path: /metadata/finalizers"))
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(finalized), finalized)).To(Succeed())
	g.Expect(finalized.Finalizers).To(BeEmpty())

	deployment := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name": "strategic", "namespace": namespace,
		},
		"spec": map[string]any{
			"replicas": int64(1),
			"selector": map[string]any{"matchLabels": map[string]any{"app": "strategic"}},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": "strategic"}},
				"spec": map[string]any{"containers": []any{
					map[string]any{"name": "app", "image": "before"},
					map[string]any{"name": "sidecar", "image": "sidecar"},
				}},
			},
		},
	}}
	g.Expect(testClient.Client.Create(ctx, deployment)).To(Succeed())
	output, err = testClient.Patch(ctx, PatchRequest{
		GVK:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Name: deployment.GetName(), Namespace: namespace, Type: "strategic",
		Patch: "spec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: after\n",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Deployment/" + namespace + "/strategic patched"))
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
	containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(containers).To(HaveLen(2))
	g.Expect(containers[0].(map[string]any)["image"]).To(Equal("after"))
	g.Expect(containers[1].(map[string]any)["name"]).To(Equal("sidecar"))

	dryRun := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "dry-run", Namespace: namespace},
		Data:       map[string]string{"value": "before"},
	}
	g.Expect(testClient.Client.Create(ctx, dryRun)).To(Succeed())
	output, err = testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: dryRun.Name, Namespace: namespace,
		Patch: "data: {value: after}", DryRun: true,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("patched (dry-run, nothing was changed)"))
	g.Expect(output).To(ContainSubstring("path: /data/value"))
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(dryRun), dryRun)).To(Succeed())
	g.Expect(dryRun.Data["value"]).To(Equal("before"))

	output, err = testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: dryRun.Name, Namespace: namespace, Patch: "data: {value: before}",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(Equal("ConfigMap/" + namespace + "/dry-run unchanged\n"))

	_, err = testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: "missing", Namespace: namespace, Patch: "data: {}",
	})
	g.Expect(err).To(MatchError("ConfigMap/" + namespace + "/missing not found"))
}

func TestPatchFluxGuardAndStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	configMapGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}

	managed := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "managed", Namespace: namespace,
			Labels: map[string]string{
				"kustomize.toolkit.fluxcd.io/name": "apps", "kustomize.toolkit.fluxcd.io/namespace": namespace,
			},
		},
		Data: map[string]string{"value": "before"},
	}
	g.Expect(testClient.Client.Create(ctx, managed)).To(Succeed())
	_, err := testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: managed.Name, Namespace: namespace, Patch: "data: {value: after}",
	})
	g.Expect(err).To(MatchError("ConfigMap/managed is managed by Flux, set overwrite to patch it"))
	output, err := testClient.Patch(ctx, PatchRequest{
		GVK: configMapGVK, Name: managed.Name, Namespace: namespace,
		Patch: "data: {value: after}", Overwrite: true,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("patched"))

	owner := createDiffOwner(t, "Kustomization", namespace, "status", map[string]any{"prune": false})
	originalSpec := owner.Object["spec"]
	output, err = testClient.Patch(ctx, PatchRequest{
		GVK: owner.GroupVersionKind(), Name: owner.GetName(), Namespace: namespace,
		Subresource: "status", Patch: "status:\n  observedGeneration: 7\n", Overwrite: true,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("Kustomization/" + namespace + "/status patched"))
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(owner.GroupVersionKind())
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKeyFromObject(owner), current)).To(Succeed())
	observed, found, err := unstructured.NestedInt64(current.Object, "status", "observedGeneration")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(observed).To(Equal(int64(7)))
	g.Expect(current.Object["spec"]).To(Equal(originalSpec))
}

func TestPatchSecretAndInvalidBodies(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: namespace},
		Data:       map[string][]byte{"token": []byte("before")},
	}
	g.Expect(testClient.Client.Create(ctx, secret)).To(Succeed())
	output, err := testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Patch: "data:\n  token: YWZ0ZXI=\n",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("***"))
	g.Expect(output).NotTo(ContainSubstring("YmVmb3Jl"))
	g.Expect(output).NotTo(ContainSubstring("YWZ0ZXI="))

	output, err = testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Patch: "stringData:\n  token: plain\n", DryRun: true,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("patched (dry-run, nothing was changed)"))
	g.Expect(output).To(ContainSubstring("*** (after)"))
	g.Expect(output).NotTo(ContainSubstring("plain"))
	g.Expect(output).NotTo(ContainSubstring("YWZ0ZXI="))

	_, err = testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Patch: "data:\n  bad key: YWZ0ZXI=\n",
	})
	g.Expect(err).To(MatchError("unable to patch Secret/" + namespace + "/secret error: Invalid"))

	_, err = testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Patch: "- op: remove\n  path: /data/token\n",
	})
	g.Expect(err).To(MatchError("patch must be a YAML/JSON object"))

	_, err = testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Type: "json", Patch: "data: {}",
	})
	g.Expect(err).To(MatchError("patch must be a list of JSON patch operations"))

	_, err = testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Name: secret.Name, Namespace: namespace,
		Patch: "data: [",
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("unable to parse patch"))
}

func TestPatchFieldsOutsideManifestSurviveApply(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	ownerLabels := map[string]string{
		"kustomize.toolkit.fluxcd.io/name":      "apps",
		"kustomize.toolkit.fluxcd.io/namespace": namespace,
	}
	manifest := configMapUnstructured(namespace, "restart", ownerLabels, map[string]any{"keep": "one"})

	// Mirror the kustomize-controller apply: kubectl-prefixed managers are cleaned up before every apply.
	rm := ssa.NewResourceManager(testClient.Client, testClient.poller,
		ssa.Owner{Field: "kustomize-controller", Group: "kustomize.toolkit.fluxcd.io"})
	opts := ssa.DefaultApplyOptions()
	opts.Cleanup = ssa.ApplyCleanupOptions{FieldManagers: []ssa.FieldManager{
		{Name: "kubectl", OperationType: metav1.ManagedFieldsOperationApply},
		{Name: "kubectl", OperationType: metav1.ManagedFieldsOperationUpdate},
	}}
	_, err := rm.Apply(ctx, manifest.DeepCopy(), opts)
	g.Expect(err).NotTo(HaveOccurred())

	output, err := testClient.Patch(ctx, PatchRequest{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, Name: "restart", Namespace: namespace,
		Overwrite: true,
		Patch:     "metadata:\n  annotations:\n    kubectl.kubernetes.io/restartedAt: \"2026-08-28T20:19:47Z\"\ndata:\n  keep: two\n",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("ConfigMap/" + namespace + "/restart patched"))

	_, err = rm.Apply(ctx, manifest.DeepCopy(), opts)
	g.Expect(err).NotTo(HaveOccurred())

	live := &corev1.ConfigMap{}
	g.Expect(testClient.Client.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: "restart"}, live)).To(Succeed())
	g.Expect(live.Annotations).To(HaveKeyWithValue("kubectl.kubernetes.io/restartedAt", "2026-08-28T20:19:47Z"))
	g.Expect(live.Data).To(HaveKeyWithValue("keep", "one"))
	managers := make([]string, 0, len(live.ManagedFields))
	for _, entry := range live.ManagedFields {
		managers = append(managers, entry.Manager)
	}
	g.Expect(managers).To(ContainElement(patchFieldManager))
}

func TestPatchErrorMessages(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	namespace := createDiffNamespace(t)
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "errors", Namespace: namespace},
		Data:       map[string]string{"a": "b"},
	}
	g.Expect(testClient.Client.Create(ctx, cm)).To(Succeed())

	_, err := testClient.Patch(ctx, PatchRequest{GVK: gvk, Name: cm.Name, Namespace: namespace, Patch: "data: oops"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(HavePrefix("unable to patch ConfigMap/" + namespace + "/errors error: patch: Invalid value: json: cannot unmarshal"))
	g.Expect(err.Error()).NotTo(ContainSubstring("managedFields"))
	g.Expect(err.Error()).NotTo(ContainSubstring(`"{`))

	_, err = testClient.Patch(ctx, PatchRequest{GVK: gvk, Name: cm.Name, Namespace: namespace, Type: "json",
		Patch: "- op: add\n  path: /spec/foo/bar\n  value: x\n"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("the server rejected our request"))
	g.Expect(err.Error()).To(ContainSubstring("/spec/foo/bar"))

	_, err = testClient.Patch(ctx, PatchRequest{GVK: gvk, Name: cm.Name, Namespace: namespace, Type: "json",
		Patch: "- op: test\n  path: /data/a\n  value: zzz\n"})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("/data/a"))
	g.Expect(err.Error()).To(ContainSubstring("failed"))
}
