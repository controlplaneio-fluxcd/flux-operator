// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/internal/inventory"
)

// WorkloadRef represents a Kubernetes workload (Deployment, StatefulSet,
// DaemonSet or CronJob) managed by a Flux applier reconciler, extracted from
// the reconciler's inventory. It carries the owning reconciler's reference and
// status so the workloads index can be served without extra cluster queries.
type WorkloadRef struct {
	// Kind is the workload kind: Deployment, StatefulSet, DaemonSet or CronJob.
	Kind string `json:"kind"`

	// Name is the name of the workload.
	Name string `json:"name"`

	// Namespace is the namespace of the workload.
	Namespace string `json:"namespace"`

	// APIVersion is the workload apiVersion (apps/v1 or batch/v1).
	APIVersion string `json:"apiVersion"`

	// ReconcilerKind is the kind of the owning Flux applier reconciler.
	ReconcilerKind string `json:"reconcilerKind"`

	// ReconcilerNamespace is the namespace of the owning Flux applier reconciler.
	ReconcilerNamespace string `json:"reconcilerNamespace"`

	// ReconcilerName is the name of the owning Flux applier reconciler.
	ReconcilerName string `json:"reconcilerName"`

	// ReconcilerStatus is the owning reconciler's status, one of:
	// "Ready", "Failed", "Progressing", "Suspended", "Unknown".
	// It drives a status badge only and is never a free-form message.
	ReconcilerStatus string `json:"reconcilerStatus"`

	// LastReconciled is the timestamp of the owning reconciler's last
	// reconciliation.
	LastReconciled metav1.Time `json:"lastReconciled"`
}

// workloadKinds is the set of Kubernetes workload kinds extracted from
// reconciler inventories for the workloads index.
var workloadKinds = map[string]struct{}{
	"Deployment":  {},
	"StatefulSet": {},
	"DaemonSet":   {},
	"CronJob":     {},
}

// extractWorkloads parses the inventory of a Flux applier reconciler (the given
// unstructured object) and returns the workloads it manages, stamped with the
// reconciler's reference and the provided ResourceStatus.
//
// Appliers targeting a remote cluster (spec.kubeConfig set) are skipped by the
// caller, since their workloads do not exist on the local cluster.
func extractWorkloads(item unstructured.Unstructured, rs ResourceStatus) []WorkloadRef {
	entries, err := inventory.FromUnstructured(&item)
	if err != nil || len(entries) == 0 {
		return nil
	}

	objects, err := inventory.List(&fluxcdv1.ResourceInventory{Entries: entries})
	if err != nil {
		return nil
	}

	result := make([]WorkloadRef, 0)
	for _, obj := range objects {
		if _, ok := workloadKinds[obj.GetKind()]; !ok {
			continue
		}

		// Only keep apps/* and batch/* workloads, skipping unrelated kinds
		// that happen to share a name (e.g. a CRD named "CronJob").
		group := obj.GroupVersionKind().Group
		if group != "apps" && group != "batch" {
			continue
		}

		result = append(result, WorkloadRef{
			Kind:                obj.GetKind(),
			Name:                obj.GetName(),
			Namespace:           obj.GetNamespace(),
			APIVersion:          obj.GetAPIVersion(),
			ReconcilerKind:      rs.Kind,
			ReconcilerNamespace: rs.Namespace,
			ReconcilerName:      rs.Name,
			ReconcilerStatus:    rs.Status,
			LastReconciled:      rs.LastReconciled,
		})
	}

	return result
}

// hasRemoteKubeConfig reports whether the given reconciler targets a remote
// cluster via spec.kubeConfig. Such appliers manage workloads that do not exist
// on the local cluster and must be skipped during inventory extraction.
func hasRemoteKubeConfig(item unstructured.Unstructured) bool {
	_, found, _ := unstructured.NestedFieldCopy(item.Object, "spec", "kubeConfig")
	return found
}

// reconcilerManagesWorkloads reports whether the given reconciler kind can carry
// a workload inventory. ResourceSetInputProvider has no inventory.
func reconcilerManagesWorkloads(kind string) bool {
	switch kind {
	case fluxcdv1.FluxKustomizationKind,
		fluxcdv1.FluxHelmReleaseKind,
		fluxcdv1.ResourceSetKind,
		fluxcdv1.FluxInstanceKind:
		return true
	default:
		return false
	}
}

// isRemoteApplier reports whether the given reconciler kind can target a remote
// cluster via spec.kubeConfig. Only Kustomization and HelmRelease support it;
// ResourceSet and FluxInstance always target the local cluster.
func isRemoteApplier(kind string) bool {
	return kind == fluxcdv1.FluxKustomizationKind || kind == fluxcdv1.FluxHelmReleaseKind
}
