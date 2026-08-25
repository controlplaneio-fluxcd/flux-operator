// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"strings"
)

// Instructions returns the server-level instructions advertised to MCP clients
// in the initialize result. Agent harnesses inject these instructions into the
// model context, while tool descriptions may be lazy loaded, so the text names
// the available tools and explains how to sequence them. The instructions are
// tailored to the tools that RegisterTools would register for the given mode.
func (m *Manager) Instructions(inCluster bool) string {
	has := func(tool string) bool {
		return m.shouldRegisterTool(tool, inCluster)
	}

	var b strings.Builder
	if m.readOnly {
		b.WriteString("This server connects to Kubernetes API to inspect and troubleshoot " +
			"workloads and the GitOps pipelines run by Flux CD. It runs in read-only mode, only tools that read from the cluster are available.\n")
	} else {
		b.WriteString("This server connects to Kubernetes API to inspect, troubleshoot and manage " +
			"workloads and the GitOps pipelines run by Flux CD.\n")
	}

	b.WriteString("\nWorkflow:\n")
	if has(ToolGetFluxInstance) {
		b.WriteString("- To check if Flux is installed and learn the Flux Operator status, " +
			"the installed controllers and the apiVersion of the Flux CRDs, call " + ToolGetFluxInstance + ".\n")
	}
	if has(ToolGetKubernetesResources) {
		b.WriteString("- Read Kubernetes and Flux resources with " + ToolGetKubernetesResources + ".")
		if has(ToolGetKubernetesAPIVersions) {
			b.WriteString(" Never guess an apiVersion, call " + ToolGetKubernetesAPIVersions + " when it is unknown.")
		}
		b.WriteString("\n")
		b.WriteString("- When listing many resources or when only some fields matter, set the fields parameter of " +
			ToolGetKubernetesResources + " to kubectl JSONPath expressions " +
			"(e.g. spec.chart.spec.version, status.conditions[?(@.type==\"Ready\")].message) to reduce the result size. " +
			"Include status.events and status.inventory when the events or the inventory are needed.\n")
		b.WriteString("- Resources managed by Flux carry labels containing fluxcd that identify " +
			"the Kustomization, HelmRelease or ResourceSet managing them.\n")
	}
	if has(ToolGetKubernetesLogs) {
		b.WriteString("- To read pod logs, get the workload with " + ToolGetKubernetesResources +
			", list its pods using the matchLabels from the workload spec, then call " +
			ToolGetKubernetesLogs + " with the pod, container and namespace.\n")
	}
	if has(ToolGetKubernetesMetrics) {
		b.WriteString("- To check the CPU and memory usage of pods, call " + ToolGetKubernetesMetrics + ".\n")
	}
	if has(ToolGetKubeConfigContexts) && has(ToolSetKubeConfigContext) {
		b.WriteString("- To work with a different cluster, call " + ToolGetKubeConfigContexts +
			" to find the context, switch to it with " + ToolSetKubeConfigContext)
		if has(ToolGetFluxInstance) {
			b.WriteString(", then call " + ToolGetFluxInstance + " again")
		}
		b.WriteString(".\n")
	}

	var actions []string
	if has(ToolApplyKubernetesManifest) {
		actions = append(actions, "- To create or update resources, generate a Kubernetes multi-doc YAML and apply it with "+
			ToolApplyKubernetesManifest+". Avoid changing Flux-managed resources directly unless explicitly asked.\n")
	}
	var reconcilers []string
	for _, tool := range []string{
		ToolReconcileFluxSource,
		ToolReconcileFluxKustomization,
		ToolReconcileFluxHelmRelease,
		ToolReconcileFluxResourceSet,
	} {
		if has(tool) {
			reconcilers = append(reconcilers, tool)
		}
	}
	if len(reconcilers) > 0 {
		line := "- To trigger a sync, call " + joinOr(reconcilers)
		if has(ToolGetKubernetesResources) {
			line += ", then verify the outcome with " + ToolGetKubernetesResources
		}
		actions = append(actions, line+".\n")
	}
	if has(ToolSuspendFluxReconciliation) && has(ToolResumeFluxReconciliation) {
		actions = append(actions, "- To pause and unpause the reconciliation of a Flux resource, call "+
			ToolSuspendFluxReconciliation+" and "+ToolResumeFluxReconciliation+".\n")
	}
	if has(ToolDeleteKubernetesResource) {
		actions = append(actions, "- To remove resources from the cluster, call "+ToolDeleteKubernetesResource+".\n")
	}
	if has(ToolInstallFluxInstance) {
		actions = append(actions, "- To bootstrap Flux on a cluster without it, call "+ToolInstallFluxInstance+".\n")
	}
	if len(actions) > 0 {
		b.WriteString("\nActions:\n")
		for _, a := range actions {
			b.WriteString(a)
		}
	}

	if has(ToolSearchFluxDocs) {
		b.WriteString("\nWhen generating or reviewing Flux resource definitions, call " + ToolSearchFluxDocs +
			" with a query that names the kind (e.g. HelmRelease valuesFrom) to read the API docs.\n")
	}

	return strings.TrimSpace(b.String())
}

// joinOr joins the items with commas and an "or" before the last one.
func joinOr(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}
