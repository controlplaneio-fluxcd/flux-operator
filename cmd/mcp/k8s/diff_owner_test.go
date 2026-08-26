// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"context"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/ssa"
	ssaerrors "github.com/fluxcd/pkg/ssa/errors"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsMissingNamespaceError(t *testing.T) {
	g := NewWithT(t)
	object := &unstructured.Unstructured{}
	object.SetAPIVersion("v1")
	object.SetKind("ConfigMap")
	object.SetName("backend")
	object.SetNamespace("apps-qa")

	missingNamespace := ssaerrors.NewDryRunErr(
		apierrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "apps-qa"), object)
	g.Expect(isMissingNamespaceError(missingNamespace, "apps-qa")).To(BeTrue())

	missingObject := ssaerrors.NewDryRunErr(
		apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "backend"), object)
	g.Expect(isMissingNamespaceError(missingObject, "apps-qa")).To(BeFalse())
}

func TestDiffRequestValidationAndGenerateNameDecoding(t *testing.T) {
	g := NewWithT(t)
	client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(), nil, meta.NewDefaultRESTMapper(nil))
	_, err := client.Diff(context.Background(), DiffRequest{Manifest: "---\n"})
	g.Expect(err).To(MatchError("no Kubernetes objects found in manifest"))

	objects, err := readDiffObjects(strings.NewReader(`Pulled: registry.example.com/charts/app:1.0.0
Digest: sha256:0123456789abcdef
---
apiVersion: v1
kind: ConfigMap
metadata:
  generateName: generated-
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - app.yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: named
`))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(2))
	g.Expect(objects[0].GetGenerateName()).To(Equal("generated-"))
	g.Expect(objects[1].GetName()).To(Equal("named"))
}

func TestResolveDiffOwnerAndOptions(t *testing.T) {
	g := NewWithT(t)
	live := ownerObject("Kustomization", "live", "flux-system", false)
	live.Object["spec"].(map[string]any)["force"] = true
	live.Object["spec"].(map[string]any)["prune"] = true
	live.Object["spec"].(map[string]any)["ignore"] = []any{map[string]any{
		"paths":  []any{"/spec/replicas"},
		"target": map[string]any{"kind": "Deployment", "name": "app"},
	}}
	client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).WithObjects(live).Build(), nil, meta.NewDefaultRESTMapper(nil))

	owner, err := client.resolveDiffOwner(context.Background(), DiffRequest{
		OwnerRef: &DiffOwnerRef{Kind: "Kustomization", Name: "live", Namespace: "flux-system"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.ssaOwner).To(Equal(ssa.Owner{Field: "kustomize-controller", Group: "kustomize.toolkit.fluxcd.io"}))
	g.Expect(owner.diffOptions.Force).To(BeTrue())
	g.Expect(owner.prune).To(BeTrue())
	g.Expect(owner.diffOptions.DriftIgnoreRules).To(HaveLen(1))
	g.Expect(owner.diffOptions.DriftIgnoreRules[0].Selector.Kind).To(Equal("Deployment"))

	future := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: future
  namespace: flux-system
spec:
  suspend: true
`
	owner, err = client.resolveDiffOwner(context.Background(), DiffRequest{
		FluxObject: future,
		OwnerRef:   &DiffOwnerRef{Kind: "Kustomization", Name: "missing", Namespace: "flux-system"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.ref.Kind).To(Equal("HelmRelease"))
	g.Expect(owner.ref.Name).To(Equal("future"))
	g.Expect(owner.futureSuspend).To(BeTrue())
	g.Expect(owner.ssaOwner).To(Equal(ssa.Owner{Field: "helm-controller", Group: "helm.toolkit.fluxcd.io"}))
	g.Expect(owner.diffOptions.Exclusions).To(BeNil())
}

func TestDiffOwnerOptionsByKind(t *testing.T) {
	g := NewWithT(t)
	resourceSet := ownerObject("ResourceSet", "apps", "flux-system", false)
	resourceSet.SetAnnotations(map[string]string{"fluxcd.controlplane.io/force": "enabled"})
	owner, err := newDiffOwner(resourceSet)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.ssaOwner).To(Equal(ssa.Owner{Field: "flux-operator", Group: "resourceset.fluxcd.controlplane.io"}))
	g.Expect(owner.diffOptions.Force).To(BeTrue())
	g.Expect(owner.diffOptions.ForceSelector).To(Equal(map[string]string{"fluxcd.controlplane.io/force": "enabled"}))
	g.Expect(owner.diffOptions.Exclusions).To(BeNil())
	g.Expect(owner.prune).To(BeTrue())

	helmRelease := ownerObject("HelmRelease", "app", "flux-system", true)
	owner, err = newDiffOwner(helmRelease)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(owner.ssaOwner).To(Equal(ssa.Owner{Field: "helm-controller", Group: "helm.toolkit.fluxcd.io"}))
	g.Expect(owner.diffOptions.DriftIgnoreRules).To(HaveLen(1))
	g.Expect(owner.diffOptions.DriftIgnoreRules[0].Selector).To(BeNil())
	g.Expect(owner.diffOptions.DriftIgnoreRules[0].Paths).To(Equal([]string{
		"/metadata/labels/helm.sh~1chart",
		"/spec/template/metadata/labels/helm.sh~1chart",
		"/spec/jobTemplate/spec/template/metadata/labels/helm.sh~1chart",
	}))
	g.Expect(owner.prune).To(BeTrue())
	g.Expect(owner.futureSuspend).To(BeTrue())
}

func TestHelmReleaseMetadataIgnoreAndHooks(t *testing.T) {
	g := NewWithT(t)
	helmOwner, err := newDiffOwner(ownerObject("HelmRelease", "app", "flux-system", false))
	g.Expect(err).NotTo(HaveOccurred())

	live := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": "app",
			"labels": map[string]any{
				"stable":        "value",
				"helm.sh/chart": "app-1.0.0_deadbeef",
			},
		},
	}}
	merged := live.DeepCopy()
	merged.SetLabels(map[string]string{
		"stable":                       "value",
		"helm.sh/chart":                "app-v1.0.0",
		"app.kubernetes.io/managed-by": "Helm",
	})
	patch, err := diffPatch(live, merged, helmOwner.diffOptions.DriftIgnoreRules)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(patch).To(HaveLen(1))
	g.Expect(patch[0].Path).To(Equal("/metadata/labels/app.kubernetes.io~1managed-by"))
	g.Expect(patch[0].Value).To(Equal("Helm"))

	for _, path := range [][]string{
		{"spec", "template", "metadata", "labels", "helm.sh/chart"},
		{"spec", "jobTemplate", "spec", "template", "metadata", "labels", "helm.sh/chart"},
	} {
		liveWorkload := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1", "kind": "Deployment", "metadata": map[string]any{"name": "app"},
		}}
		mergedWorkload := liveWorkload.DeepCopy()
		g.Expect(unstructured.SetNestedField(liveWorkload.Object, "app-1.0.0_deadbeef", path...)).To(Succeed())
		g.Expect(unstructured.SetNestedField(mergedWorkload.Object, "app-v1.0.0", path...)).To(Succeed())
		patch, err = diffPatch(liveWorkload, mergedWorkload, helmOwner.diffOptions.DriftIgnoreRules)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(patch).To(BeEmpty())
	}

	hook := merged.DeepCopy()
	hook.SetAnnotations(map[string]string{"helm.sh/hook": "pre-install,post-install"})
	value, found := helmHookValue(helmOwner, hook)
	g.Expect(found).To(BeTrue())
	g.Expect(value).To(Equal("pre-install,post-install"))

	kustomizeOwner, err := newDiffOwner(ownerObject("Kustomization", "app", "flux-system", false))
	g.Expect(err).NotTo(HaveOccurred())
	_, found = helmHookValue(kustomizeOwner, hook)
	g.Expect(found).To(BeFalse())
}

func TestDiffPruneInventoryPresenceAndDecoding(t *testing.T) {
	g := NewWithT(t)
	ownerObject := ownerObject("Kustomization", "apps", "flux-system", false)
	ownerObject.Object["spec"].(map[string]any)["prune"] = true
	owner, err := newDiffOwner(ownerObject)
	g.Expect(err).NotTo(HaveOccurred())
	owner.live = ownerObject.DeepCopy()
	client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(), nil, meta.NewDefaultRESTMapper(nil))
	rm := newResourceManagerWithOwner(client.Client, client.poller, owner.ssaOwner)

	objects, status := client.diffPrune(context.Background(), owner, rm, nil)
	g.Expect(objects).To(BeEmpty())
	g.Expect(status).To(Equal("prune: unavailable"))

	g.Expect(unstructured.SetNestedField(owner.live.Object, "invalid", "status", "inventory")).To(Succeed())
	objects, status = client.diffPrune(context.Background(), owner, rm, nil)
	g.Expect(objects).To(BeEmpty())
	g.Expect(status).To(Equal("prune: unavailable"))

	owner.live = nil
	objects, status = client.diffPrune(context.Background(), owner, rm, nil)
	g.Expect(objects).To(BeEmpty())
	g.Expect(status).To(Equal("prune: skipped (owner not in cluster)"))
}

func TestResolveDiffOwnerReferenceNotFound(t *testing.T) {
	g := NewWithT(t)
	client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(), nil, meta.NewDefaultRESTMapper(nil))
	_, err := client.resolveDiffOwner(context.Background(), DiffRequest{
		OwnerRef: &DiffOwnerRef{Kind: "ResourceSet", Name: "missing", Namespace: "flux-system"},
	})
	g.Expect(err).To(MatchError("owner not found in the cluster: ResourceSet/flux-system/missing; if it is new, pass its definition in flux_object"))
}

func TestResolveDiffOwnerGuards(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		match    string
	}{
		{
			name: "multiple documents",
			manifest: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata: {name: one, namespace: flux-system}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata: {name: two, namespace: flux-system}
`,
			match: "expected exactly one document",
		},
		{
			name: "missing namespace",
			manifest: `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata: {name: app}
`,
			match: "metadata.namespace is required",
		},
		{
			name: "remote owner",
			manifest: `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata: {name: app, namespace: flux-system}
spec:
  kubeConfig: {secretRef: {name: remote}}
`,
			match: "diff not supported for owners targeting remote clusters",
		},
		{
			name: "unsupported kind",
			manifest: `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata: {name: app, namespace: flux-system}
`,
			match: "supported kinds are Kustomization, ResourceSet and HelmRelease",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(), nil, meta.NewDefaultRESTMapper(nil))
			_, err := client.resolveDiffOwner(context.Background(), DiffRequest{FluxObject: tt.manifest})
			g.Expect(err).To(MatchError(ContainSubstring(tt.match)))
		})
	}
}

func TestPrepareDiffObjectsMetadata(t *testing.T) {
	g := NewWithT(t)
	for _, kind := range []string{"Kustomization", "ResourceSet", "HelmRelease"} {
		ownerObject := ownerObject(kind, "owner", "flux-system", false)
		ownerObject.Object["spec"].(map[string]any)["commonMetadata"] = map[string]any{
			"labels":      map[string]any{"common": "value", ownerObject.GroupVersionKind().Group + "/name": "wrong"},
			"annotations": map[string]any{"note": "test"},
		}
		owner, err := newDiffOwner(ownerObject)
		g.Expect(err).NotTo(HaveOccurred())
		client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).Build(), nil, meta.NewDefaultRESTMapper(nil))
		rm := newResourceManagerWithOwner(client.Client, client.poller, owner.ssaOwner)
		objects := []*unstructured.Unstructured{{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app"},
		}}}
		g.Expect(prepareDiffObjects(context.Background(), client.Client, &objects, owner, rm)).To(Succeed())
		g.Expect(objects[0].GetLabels()["common"]).To(Equal("value"))
		g.Expect(objects[0].GetAnnotations()["note"]).To(Equal("test"))
		g.Expect(objects[0].GetLabels()[owner.ssaOwner.Group+"/name"]).To(Equal("owner"))
		g.Expect(objects[0].GetLabels()[owner.ssaOwner.Group+"/namespace"]).To(Equal("flux-system"))
	}
}

func TestGetFluxOwner(t *testing.T) {
	g := NewWithT(t)
	resource := ownerObject("ResourceSet", "resource", "flux-system", false)
	resource.SetLabels(map[string]string{
		"resourceset.fluxcd.controlplane.io/name":      "apps",
		"resourceset.fluxcd.controlplane.io/namespace": "platform",
	})
	client := NewClient(fake.NewClientBuilder().WithScheme(NewTestScheme()).WithObjects(resource).Build(), nil, meta.NewDefaultRESTMapper(nil))
	owner, ok := client.GetFluxOwner(context.Background(), resource.GroupVersionKind(), resource.GetName(), resource.GetNamespace())
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(&DiffOwnerRef{Kind: "ResourceSet", Name: "apps", Namespace: "platform"}))
}

func TestFluxOwnerFromLabels(t *testing.T) {
	g := NewWithT(t)
	owner, ok := fluxOwnerFromLabels(map[string]string{
		"resourceset.fluxcd.controlplane.io/name":      "apps",
		"resourceset.fluxcd.controlplane.io/namespace": "flux-system",
	})
	g.Expect(ok).To(BeTrue())
	g.Expect(owner).To(Equal(&DiffOwnerRef{Kind: "ResourceSet", Name: "apps", Namespace: "flux-system"}))
	_, ok = fluxOwnerFromLabels(map[string]string{"kustomize.toolkit.fluxcd.io/namespace": "flux-system"})
	g.Expect(ok).To(BeFalse())
}

func ownerObject(kind, name, namespace string, suspend bool) *unstructured.Unstructured {
	gvk, err := ownerGVK(kind)
	if err != nil {
		panic(err)
	}
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{"suspend": suspend},
	}}
	object.SetGroupVersionKind(gvk)
	return object
}
