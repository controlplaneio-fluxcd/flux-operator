// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

const (
	// ScopeReadOnly allows all read-only toolbox operations.
	ScopeReadOnly = "toolbox:read_only"
	// ScopeReadWrite allows all toolbox operations.
	ScopeReadWrite = "toolbox:read_write"

	// ScopeApplyManifest allows applying Kubernetes manifests.
	ScopeApplyManifest = "toolbox:apply_manifest"
	// ScopeDeleteResource allows deleting Kubernetes resources.
	ScopeDeleteResource = "toolbox:delete_resource"
	// ScopeGetAPIs allows listing available Kubernetes APIs.
	ScopeGetAPIs = "toolbox:get_apis"
	// ScopeGetInstance allows getting a FluxInstance resource.
	ScopeGetInstance = "toolbox:get_instance"
	// ScopeGetLogs allows retrieving logs from pods.
	ScopeGetLogs = "toolbox:get_logs"
	// ScopeGetMetrics allows retrieving metrics from the Kubernetes API.
	ScopeGetMetrics = "toolbox:get_metrics"
	// ScopeGetResource allows getting Kubernetes resources.
	ScopeGetResource = "toolbox:get_resource"
	// ScopeReconcileHelmRelease allows reconciling HelmRelease resources.
	ScopeReconcileHelmRelease = "toolbox:reconcile_helmrelease"
	// ScopeReconcileKustomization allows reconciling Kustomization resources.
	ScopeReconcileKustomization = "toolbox:reconcile_kustomization"
	// ScopeReconcileResourceSet allows reconciling ResourceSet resources.
	ScopeReconcileResourceSet = "toolbox:reconcile_resourceset"
	// ScopeReconcileSource allows reconciling Flux source resources.
	ScopeReconcileSource = "toolbox:reconcile_source"
	// ScopeResumeReconciliation allows resuming the reconciliation of Flux resources.
	ScopeResumeReconciliation = "toolbox:resume_reconciliation"
	// ScopeSuspendReconciliation allows suspending the reconciliation of Flux resources.
	ScopeSuspendReconciliation = "toolbox:suspend_reconciliation"
	// ScopeSearchFluxDocs allows searching the Flux documentation.
	ScopeSearchFluxDocs = "toolbox:search_flux_docs"
)
