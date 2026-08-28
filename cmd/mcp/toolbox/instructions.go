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
	if has(ToolGetKubernetesEvents) {
		b.WriteString("- When troubleshooting workloads, call " + ToolGetKubernetesEvents +
			" with the workload kind, name and namespace, and narrow with type Warning, since and grep.\n")
	}
	if has(ToolGetKubernetesLogs) {
		b.WriteString("- To read application logs, call " + ToolGetKubernetesLogs +
			" directly with the workload kind, name and namespace found in a Flux resource's inventory via " +
			ToolGetKubernetesResources + ". Omit container to read all regular containers; if the result is truncated, " +
			"narrow it with container and limit.\n")
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
	if has(ToolDiffKubernetesManifest) {
		line := "- Before committing GitOps changes, build the manifests locally (kustomize build <path> --load-restrictor=LoadRestrictionsNone, flux-operator build resourceset, helm template) and pass the COMPLETE output to " +
			ToolDiffKubernetesManifest
		if m.localFiles {
			line += "; for large builds, write it to a file and pass the absolute path in yaml_path"
		}
		line += ". For manifests managed by Flux, identify the owner (Kustomization, HelmRelease or ResourceSet)"
		if has(ToolGetKubernetesResources) {
			line += " that matches spec.path/sourceRef with " + ToolGetKubernetesResources
		}
		line += " and set owner_kind/owner_name/owner_namespace, or pass its YAML in flux_object if it is new or not yet on the cluster." +
			" Skip owner lookup when the user already names the owner; ad-hoc manifests need none."
		b.WriteString(line + "\n")
	}

	var actions []string
	if has(ToolApplyKubernetesManifest) {
		line := "- To create or update resources, generate a Kubernetes multi-doc YAML"
		if has(ToolDiffKubernetesManifest) {
			line += ", preview the changes with " + ToolDiffKubernetesManifest + " (no owner needed)"
		}
		actions = append(actions, line+" and apply it with "+ToolApplyKubernetesManifest+
			". Avoid changing Flux-managed resources directly unless explicitly asked.\n")
	}
	if has(ToolReconcileFluxResource) {
		line := "- To trigger a sync, call " + ToolReconcileFluxResource
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
