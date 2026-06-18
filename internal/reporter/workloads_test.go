// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func newKustomizationWithInventory(name string, entries []any, kubeConfig bool) unstructured.Unstructured {
	spec := map[string]any{}
	if kubeConfig {
		spec["kubeConfig"] = map[string]any{
			"secretRef": map[string]any{"name": "remote"},
		}
	}
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       fluxcdv1.FluxKustomizationKind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": "flux-system",
			},
			"spec": spec,
			"status": map[string]any{
				"inventory": map[string]any{
					"entries": entries,
				},
			},
		},
	}
}

func TestExtractWorkloads_FiltersKindsAndGroups(t *testing.T) {
	g := NewWithT(t)

	entries := []any{
		map[string]any{"id": "apps-ns_web_apps_Deployment", "v": "v1"},
		map[string]any{"id": "apps-ns_db_apps_StatefulSet", "v": "v1"},
		map[string]any{"id": "apps-ns_agent_apps_DaemonSet", "v": "v1"},
		map[string]any{"id": "apps-ns_backup_batch_CronJob", "v": "v1"},
		// Non-workload kinds and non-apps/batch groups must be skipped.
		map[string]any{"id": "apps-ns_web__Service", "v": "v1"},
		map[string]any{"id": "apps-ns_cfg__ConfigMap", "v": "v1"},
		map[string]any{"id": "apps-ns_job_batch_Job", "v": "v1"},
		// A CRD that happens to be named like a workload kind but in a foreign group.
		map[string]any{"id": "apps-ns_CronJob_example.com_CronJob", "v": "v1"},
	}

	item := newKustomizationWithInventory("app", entries, false)
	rs := ResourceStatus{
		Kind:      fluxcdv1.FluxKustomizationKind,
		Name:      "app",
		Namespace: "flux-system",
		Status:    StatusReady,
	}

	workloads := extractWorkloads(item, rs)
	g.Expect(workloads).To(HaveLen(4))

	kinds := map[string]bool{}
	for _, wl := range workloads {
		kinds[wl.Kind] = true
		g.Expect(wl.ReconcilerKind).To(Equal(fluxcdv1.FluxKustomizationKind))
		g.Expect(wl.ReconcilerName).To(Equal("app"))
		g.Expect(wl.ReconcilerNamespace).To(Equal("flux-system"))
		g.Expect(wl.ReconcilerStatus).To(Equal(StatusReady))
		g.Expect(wl.Namespace).To(Equal("apps-ns"))
	}
	g.Expect(kinds).To(HaveKey("Deployment"))
	g.Expect(kinds).To(HaveKey("StatefulSet"))
	g.Expect(kinds).To(HaveKey("DaemonSet"))
	g.Expect(kinds).To(HaveKey("CronJob"))

	// Verify apiVersion is set correctly per group.
	for _, wl := range workloads {
		switch wl.Kind {
		case "CronJob":
			g.Expect(wl.APIVersion).To(Equal("batch/v1"))
		default:
			g.Expect(wl.APIVersion).To(Equal("apps/v1"))
		}
	}
}

func TestExtractWorkloads_EmptyInventory(t *testing.T) {
	g := NewWithT(t)

	item := newKustomizationWithInventory("app", []any{}, false)
	g.Expect(extractWorkloads(item, ResourceStatus{})).To(BeEmpty())
}

func TestHasRemoteKubeConfig(t *testing.T) {
	g := NewWithT(t)

	local := newKustomizationWithInventory("local-app", nil, false)
	remote := newKustomizationWithInventory("remote-app", nil, true)

	g.Expect(hasRemoteKubeConfig(local)).To(BeFalse())
	g.Expect(hasRemoteKubeConfig(remote)).To(BeTrue())
}

func TestReconcilerManagesWorkloads(t *testing.T) {
	g := NewWithT(t)

	g.Expect(reconcilerManagesWorkloads(fluxcdv1.FluxKustomizationKind)).To(BeTrue())
	g.Expect(reconcilerManagesWorkloads(fluxcdv1.FluxHelmReleaseKind)).To(BeTrue())
	g.Expect(reconcilerManagesWorkloads(fluxcdv1.ResourceSetKind)).To(BeTrue())
	g.Expect(reconcilerManagesWorkloads(fluxcdv1.FluxInstanceKind)).To(BeTrue())
	g.Expect(reconcilerManagesWorkloads(fluxcdv1.ResourceSetInputProviderKind)).To(BeFalse())
	g.Expect(reconcilerManagesWorkloads(fluxcdv1.FluxGitRepositoryKind)).To(BeFalse())
}

func TestIsRemoteApplier(t *testing.T) {
	g := NewWithT(t)

	g.Expect(isRemoteApplier(fluxcdv1.FluxKustomizationKind)).To(BeTrue())
	g.Expect(isRemoteApplier(fluxcdv1.FluxHelmReleaseKind)).To(BeTrue())
	g.Expect(isRemoteApplier(fluxcdv1.ResourceSetKind)).To(BeFalse())
	g.Expect(isRemoteApplier(fluxcdv1.FluxInstanceKind)).To(BeFalse())
}

// TestGetReconcilersStatus_SkipsRemoteApplier exercises the remote-cluster skip
// guard end-to-end: a local Kustomization's workloads are extracted while a
// Kustomization targeting a remote cluster (spec.kubeConfig set) is skipped.
func TestGetReconcilersStatus_SkipsRemoteApplier(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())
	ksGVK := schema.GroupVersionKind{
		Group:   "kustomize.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    fluxcdv1.FluxKustomizationKind,
	}
	scheme.AddKnownTypeWithName(ksGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(ksGVK.GroupVersion().WithKind(ksGVK.Kind+"List"), &unstructured.UnstructuredList{})

	local := newKustomizationWithInventory("local", []any{
		map[string]any{"id": "apps_web_apps_Deployment", "v": "v1"},
	}, false)
	remote := newKustomizationWithInventory("remote", []any{
		map[string]any{"id": "apps_api_apps_Deployment", "v": "v1"},
	}, true)

	r := newTestReporter(scheme, &local, &remote)

	crds := []metav1.GroupVersionKind{{Group: ksGVK.Group, Version: ksGVK.Version, Kind: ksGVK.Kind}}
	_, _, _, workloads, err := r.getReconcilersStatus(ctx, crds)
	g.Expect(err).ToNot(HaveOccurred())

	// Only the local Kustomization's workload is extracted; the remote one is skipped.
	g.Expect(workloads).To(HaveLen(1))
	g.Expect(workloads[0].Name).To(Equal("web"))
	g.Expect(workloads[0].ReconcilerName).To(Equal("local"))
}

// TestGetOperatorReconcilersStatus_IncludesFluxInstanceControllers verifies that
// the Flux controllers' own Deployments, recorded in the FluxInstance inventory,
// surface as workloads through the operator reconcilers pass.
func TestGetOperatorReconcilersStatus_IncludesFluxInstanceControllers(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(fluxcdv1.AddToScheme(scheme)).To(Succeed())

	instance := &fluxcdv1.FluxInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "flux",
			Namespace: "flux-system",
		},
		Status: fluxcdv1.FluxInstanceStatus{
			Inventory: &fluxcdv1.ResourceInventory{
				Entries: []fluxcdv1.ResourceRef{
					{ID: "flux-system_source-controller_apps_Deployment", Version: "v1"},
					{ID: "flux-system_kustomize-controller_apps_Deployment", Version: "v1"},
				},
			},
		},
	}

	r := newTestReporter(scheme, instance)

	_, _, _, workloads, err := r.getOperatorReconcilersStatus(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	byName := map[string]string{}
	for _, wl := range workloads {
		byName[wl.Name] = wl.ReconcilerKind
		g.Expect(wl.Kind).To(Equal("Deployment"))
		g.Expect(wl.Namespace).To(Equal("flux-system"))
	}
	g.Expect(byName).To(HaveKeyWithValue("source-controller", fluxcdv1.FluxInstanceKind))
	g.Expect(byName).To(HaveKeyWithValue("kustomize-controller", fluxcdv1.FluxInstanceKind))
}
