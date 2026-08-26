// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKustomizationDeferredTransforms(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	ownerObject := ownerObject("Kustomization", "apps", "flux-system", false)
	ownerObject.Object["spec"].(map[string]any)["namePrefix"] = "preview-"
	ownerObject.Object["spec"].(map[string]any)["components"] = []any{"../component"}
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []*unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app"},
	}}}
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).Build()
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetName()).To(Equal("preview-app"))
	g.Expect(owner.warnings).To(Equal([]string{"spec.components not applied, diff may be incomplete"}))

	output, err := renderDiff(diffResult{
		Owner:        owner.ref,
		FieldManager: owner.ssaOwner.Field,
		Warnings:     owner.warnings,
		Objects:      []diffObjectResult{{Subject: "ConfigMap/preview-app", State: diffStateUnchanged}},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(output).To(ContainSubstring("spec.components not applied, diff may be incomplete\n\nNo changes detected"))
}

func TestKustomizationBuildTransforms(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).Build()
	ownerObject := ownerObject("Kustomization", "apps", "flux-system", false)
	spec := ownerObject.Object["spec"].(map[string]any)
	spec["targetNamespace"] = "apps"
	spec["nameSuffix"] = "-v2"
	spec["patches"] = []any{
		map[string]any{
			"target": map[string]any{"kind": "Deployment", "name": "app"},
			"patch":  "- op: replace\n  path: /spec/replicas\n  value: 2\n",
		},
	}
	spec["images"] = []any{
		map[string]any{"name": "ghcr.io/example/app", "newName": "registry.example.com/app", "digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())

	objects := []*unstructured.Unstructured{
		{Object: map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "app"},
			"spec": map[string]any{
				"replicas": int64(1),
				"template": map[string]any{"spec": map[string]any{"containers": []any{
					map[string]any{"name": "app", "image": "ghcr.io/example/app:1.0.0"},
				}}},
			},
		}},
		{Object: map[string]any{
			"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole", "metadata": map[string]any{"name": "reader"},
		}},
	}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects).To(HaveLen(2))
	g.Expect(objects[0].GetKind()).To(Equal("Deployment"))
	g.Expect(objects[0].GetName()).To(Equal("app-v2"))
	g.Expect(objects[0].GetNamespace()).To(Equal("apps"))
	replicas, _, _ := unstructured.NestedInt64(objects[0].Object, "spec", "replicas")
	g.Expect(replicas).To(Equal(int64(2)))
	containers, _, _ := unstructured.NestedSlice(objects[0].Object, "spec", "template", "spec", "containers")
	g.Expect(containers[0].(map[string]any)["image"]).To(Equal("registry.example.com/app@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))
	g.Expect(objects[1].GetKind()).To(Equal("ClusterRole"))
	g.Expect(objects[1].GetName()).To(Equal("reader-v2"))
	g.Expect(objects[1].GetNamespace()).To(BeEmpty())
}

func TestKustomizationBuildMetadataIgnoreRules(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	object := ownerObject("Kustomization", "apps", "flux-system", false)
	object.Object["spec"].(map[string]any)["buildMetadata"] = []any{"originAnnotations", "transformerAnnotations", "managedByLabel"}
	owner, err := newDiffOwner(object)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.diffOptions.DriftIgnoreRules).To(HaveLen(1))
	g.Expect(owner.diffOptions.DriftIgnoreRules[0].Paths).To(ConsistOf(
		"/metadata/annotations/config.kubernetes.io~1origin",
		"/metadata/annotations/config.kubernetes.io~1transformations",
		"/metadata/labels/app.kubernetes.io~1managed-by",
	))

	// buildMetadata alone does not trigger a second build and leaves the objects untouched.
	g.Expect(requiresKustomizationBuild(object)).To(BeFalse())
	objects := []*unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app"},
	}}}
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).Build()
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetAnnotations()).To(BeEmpty())
	g.Expect(objects[0].GetLabels()).To(BeEmpty())

	// A second build for other transforms does not apply buildMetadata either.
	object.Object["spec"].(map[string]any)["namePrefix"] = "preview-"
	owner, err = newDiffOwner(object)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects[0].GetName()).To(Equal("preview-app"))
	g.Expect(objects[0].GetAnnotations()).To(BeEmpty())
	g.Expect(objects[0].GetLabels()).To(BeEmpty())

	// Only the configured options are ignored.
	object = ownerObject("Kustomization", "apps", "flux-system", false)
	object.Object["spec"].(map[string]any)["buildMetadata"] = []any{"originAnnotations"}
	owner, err = newDiffOwner(object)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.diffOptions.DriftIgnoreRules).To(HaveLen(1))
	g.Expect(owner.diffOptions.DriftIgnoreRules[0].Paths).To(Equal([]string{
		"/metadata/annotations/config.kubernetes.io~1origin",
	}))

	object = ownerObject("Kustomization", "apps", "flux-system", false)
	owner, err = newDiffOwner(object)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.diffOptions.DriftIgnoreRules).To(BeEmpty())
}

func TestKustomizationPostBuildSubstitution(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	source := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "vars", Namespace: "flux-system"},
		Data:       map[string]string{"cluster": "staging"},
	}
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).WithObjects(source).Build()
	object := ownerObject("Kustomization", "apps", "flux-system", false)
	object.Object["spec"].(map[string]any)["postBuild"] = map[string]any{
		"substituteFrom": []any{map[string]any{"kind": "ConfigMap", "name": "vars"}},
	}
	owner, err := newDiffOwner(object)
	g.Expect(err).NotTo(HaveOccurred())
	objects := []*unstructured.Unstructured{{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app"},
		"data": map[string]any{"value": "${cluster}"},
	}}}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	value, _, _ := unstructured.NestedString(objects[0].Object, "data", "value")
	g.Expect(value).To(Equal("staging"))

	objects[0].Object["data"] = map[string]any{"value": "${missing}"}
	err = applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)
	g.Expect(err).To(MatchError(ContainSubstring("missing")))

	owner.object.Object["spec"].(map[string]any)["postBuild"] = map[string]any{
		"substituteFrom": []any{map[string]any{"kind": "ConfigMap", "name": "absent"}},
	}
	objects[0].Object["data"] = map[string]any{"value": "${cluster}"}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	value, _, _ = unstructured.NestedString(objects[0].Object, "data", "value")
	g.Expect(value).To(Equal("${cluster}"))
	g.Expect(owner.warnings).To(ContainElement(
		"substituteFrom ConfigMap flux-system/absent not found in the cluster, variables left unresolved"))

	owner.object.Object["spec"].(map[string]any)["postBuild"] = map[string]any{
		"substituteFrom": []any{map[string]any{"kind": "ConfigMap", "name": "absent", "optional": true}},
		"substitute":     map[string]any{"fallback": "available"},
	}
	objects[0].Object["data"] = map[string]any{"value": "${fallback}"}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	value, _, _ = unstructured.NestedString(objects[0].Object, "data", "value")
	g.Expect(value).To(Equal("available"))
}

func TestResourceSetCopyFromTransform(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "apps"}, Data: map[string]string{"key": "value"}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "apps"}, Data: map[string][]byte{"token": []byte("secret")}}
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).WithObjects(cm, secret).Build()
	ownerObject := ownerObject("ResourceSet", "apps", "flux-system", false)
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	objects := []*unstructured.Unstructured{
		{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{
			"name": "copy", "annotations": map[string]any{"fluxcd.controlplane.io/copyFrom": "apps/source"},
		}}},
		{Object: map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{
			"name": "copy", "annotations": map[string]any{"fluxcd.controlplane.io/copyFrom": "apps/source"},
		}}},
	}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	data, _, _ := unstructured.NestedStringMap(objects[0].Object, "data")
	g.Expect(data).To(Equal(map[string]string{"key": "value"}))
	stringData, _, _ := unstructured.NestedStringMap(objects[1].Object, "stringData")
	g.Expect(stringData).To(Equal(map[string]string{"token": "secret"}))

	missing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{
			"name": "missing-copy", "annotations": map[string]any{"fluxcd.controlplane.io/copyFrom": "apps/absent"},
		},
	}}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &[]*unstructured.Unstructured{missing}, owner)).To(Succeed())
	_, found, err := unstructured.NestedMap(missing.Object, "data")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeFalse())
	g.Expect(owner.warnings).To(ContainElement(
		"copyFrom source ConfigMap apps/absent not found in the cluster, data left empty"))
}

func TestHelmReleaseMetadataAndNamespace(t *testing.T) {
	g := NewWithT(t)
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}, {Group: "apiextensions.k8s.io", Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}, meta.RESTScopeRoot)
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).WithRESTMapper(mapper).Build()
	ownerObject := ownerObject("HelmRelease", "backend", "flux-system", false)
	ownerObject.Object["spec"].(map[string]any)["targetNamespace"] = "apps"
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	objects := []*unstructured.Unstructured{
		{Object: map[string]any{
			"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition",
			"metadata": map[string]any{"name": "widgets.example.com"},
			"spec":     map[string]any{"group": "example.com", "scope": "Namespaced", "names": map[string]any{"kind": "Widget"}},
		}},
		{Object: map[string]any{"apiVersion": "example.com/v1", "kind": "Widget", "metadata": map[string]any{"name": "app"}}},
		{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app"}}},
	}
	rm := newResourceManagerWithOwner(kubeClient, nil, owner.ssaOwner)
	g.Expect(prepareDiffObjects(context.Background(), kubeClient, &objects, owner, rm)).To(Succeed())
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("helm.toolkit.fluxcd.io/name", "backend"))
	g.Expect(objects[0].GetLabels()).NotTo(HaveKey("app.kubernetes.io/managed-by"))
	g.Expect(objects[0].GetAnnotations()).NotTo(HaveKey("meta.helm.sh/release-name"))
	g.Expect(objects[0].GetNamespace()).To(BeEmpty())
	liveTemplateCRD := objects[0].DeepCopy()
	liveTemplateCRD.SetAnnotations(map[string]string{"meta.helm.sh/release-name": "old-release"})
	applyHelmMetadataForLiveCRD(objects[0], liveTemplateCRD, owner)
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "Helm"))
	g.Expect(objects[0].GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-name", "apps-backend"))
	g.Expect(objects[0].GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-namespace", "apps"))
	crdsDirectoryCRD := objects[0].DeepCopy()
	crdsDirectoryCRD.SetLabels(map[string]string{"helm.toolkit.fluxcd.io/name": "backend"})
	crdsDirectoryCRD.SetAnnotations(nil)
	applyHelmMetadataForLiveCRD(crdsDirectoryCRD, crdsDirectoryCRD.DeepCopy(), owner)
	g.Expect(crdsDirectoryCRD.GetLabels()).NotTo(HaveKey("app.kubernetes.io/managed-by"))
	g.Expect(crdsDirectoryCRD.GetAnnotations()).NotTo(HaveKey("meta.helm.sh/release-name"))
	for _, object := range objects[1:] {
		g.Expect(object.GetNamespace()).To(Equal("apps"))
		g.Expect(object.GetLabels()).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "Helm"))
		g.Expect(object.GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-name", "apps-backend"))
		g.Expect(object.GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-namespace", "apps"))
	}
}

func TestHelmReleasePostRenderers(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	kubeClient := fake.NewClientBuilder().WithScheme(NewTestScheme()).Build()
	deployment := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment",
			"metadata": map[string]any{"name": "app", "labels": map[string]any{"app": "backend"}},
			"spec": map[string]any{
				"replicas": int64(1),
				"template": map[string]any{"spec": map[string]any{"containers": []any{
					map[string]any{"name": "app", "image": "ghcr.io/example/app:1.0.0"},
				}}},
			},
		}}
	}
	service := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "app"},
		}}
	}

	// No post-renderers leaves the objects untouched.
	ownerObject := ownerObject("HelmRelease", "backend", "flux-system", false)
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	objects := []*unstructured.Unstructured{deployment(), service()}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects).To(HaveLen(2))
	g.Expect(objects[0].Object).To(Equal(deployment().Object))

	// Post-renderers are applied in order with patches and images.
	ownerObject.Object["spec"].(map[string]any)["postRenderers"] = []any{
		map[string]any{"kustomize": map[string]any{
			"patches": []any{
				map[string]any{
					"target": map[string]any{"kind": "Deployment", "labelSelector": "app=backend"},
					"patch":  "- op: replace\n  path: /spec/replicas\n  value: 3\n",
				},
				map[string]any{
					"patch": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n  labels:\n    tier: web\n",
				},
			},
			"images": []any{
				map[string]any{"name": "ghcr.io/example/app", "newTag": "2.0.0"},
			},
		}},
		map[string]any{"kustomize": map[string]any{
			"patches": []any{
				map[string]any{
					"target": map[string]any{"kind": "Deployment", "labelSelector": "tier=web"},
					"patch":  "- op: add\n  path: /metadata/annotations\n  value:\n    rendered: second\n",
				},
			},
		}},
	}
	owner, err = newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	objects = []*unstructured.Unstructured{deployment(), service()}
	g.Expect(applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)).To(Succeed())
	g.Expect(objects).To(HaveLen(2))
	g.Expect(objects[0].GetKind()).To(Equal("Deployment"))
	replicas, _, _ := unstructured.NestedInt64(objects[0].Object, "spec", "replicas")
	g.Expect(replicas).To(Equal(int64(3)))
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("tier", "web"))
	g.Expect(objects[0].GetAnnotations()).To(HaveKeyWithValue("rendered", "second"))
	containers, _, _ := unstructured.NestedSlice(objects[0].Object, "spec", "template", "spec", "containers")
	g.Expect(containers[0].(map[string]any)["image"]).To(Equal("ghcr.io/example/app:2.0.0"))
	g.Expect(objects[1].Object).To(Equal(service().Object))

	// Invalid patches surface the renderer index.
	ownerObject.Object["spec"].(map[string]any)["postRenderers"] = []any{
		map[string]any{"kustomize": map[string]any{
			"patches": []any{map[string]any{"patch": "not: [valid"}},
		}},
	}
	owner, err = newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	objects = []*unstructured.Unstructured{deployment()}
	err = applyDeferredOwnerTransforms(ctx, kubeClient, &objects, owner)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(HavePrefix("postRenderers[0] failed"))
}

func TestHelmReleaseNameShortening(t *testing.T) {
	g := NewWithT(t)
	name := "release-name-with-very-long-name-which-is-longer-than-53-characters"
	g.Expect(shortenHelmReleaseName(name)).To(Equal("release-name-with-very-long-name-which-i-788ca0d0d7b0"))
	g.Expect(shortenHelmReleaseName("short")).To(Equal("short"))
}

func TestKustomizationFieldsAreAccepted(t *testing.T) {
	g := NewWithT(t)
	object := ownerObject("Kustomization", "apps", "flux-system", false)
	object.Object["spec"].(map[string]any)["namePrefix"] = "preview-"
	object.Object["spec"].(map[string]any)["components"] = []any{"component"}
	object.Object["spec"].(map[string]any)["decryption"] = map[string]any{"provider": "sops"}
	g.Expect(validateDiffOwner(object)).To(Succeed())
	g.Expect(strings.Join([]string{object.GetKind(), object.GetName()}, "/")).To(Equal("Kustomization/apps"))
}
