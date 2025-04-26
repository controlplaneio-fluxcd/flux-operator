// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
)

type Prompt struct {
	Name        string
	Description string
	Handler     any
}

var PromptList = []Prompt{
	{
		Name:        "debug_flux_kustomization",
		Description: "Troubleshoot a Kustomization resource",
		Handler:     DebugFluxKustomizationHandler,
	},
	{
		Name:        "debug_flux_helmrelease",
		Description: "Troubleshoot a HelmRelease resource",
		Handler:     DebugFluxHelmReleaseHandler,
	},
}

type DebugFluxReconciliationArgs struct {
	Name      string `json:"name" jsonschema:"description=The name of the Flux resource."`
	Namespace string `json:"namespace" jsonschema:"description=The namespace of the Flux resource."`
	Cluster   string `json:"cluster" jsonschema:"description=The context name of the cluster."`
}

func DebugFluxHelmReleaseHandler(args DebugFluxReconciliationArgs) (*mcpgolang.PromptResponse, error) {
	cluster := "current"
	if args.Cluster != "" {
		cluster = args.Cluster
	}

	return mcpgolang.NewPromptResponse(
		"Debug instructions for a HelmRelease",
		mcpgolang.NewPromptMessage(
			mcpgolang.NewTextContent(fmt.Sprintf(`
To debug the Flux HelmRelease %s in namespace %s on the %s cluster, follow these steps:

0. Use the get_kubeconfig_contexts tool to find the context name for the %[3]s cluster
and use the set_kubeconfig_context to change the context to it.
1. Use the get_flux_instance_report tool to check the helm-controller deployment
status and the available Flux API versions.
The instance report will also show any issues with helm-controller deployment.
2. Retrieve the HelmRelease details using the get_kubernetes_resources tool.
3. Identify the HelmRelease source by looking at the spec.chartRef or the spec.chart field,
then use the get_kubernetes_resources tool to fetch the corresponding
OCIRepository, HelmChart, or GitRepository resource.
4. If the HelmRelease is in a failed state or in progress, check the status
conditions and events for any error messages.
5. Use the get_kubernetes_resources tool to check the status of the resources
found in the HelmRelease status inventory.
6. Write a detailed report of the issue, including the release spec, status, and any error messages.

`, args.Name, args.Namespace, cluster)),
			mcpgolang.RoleAssistant,
		),
	), nil
}

func DebugFluxKustomizationHandler(args DebugFluxReconciliationArgs) (*mcpgolang.PromptResponse, error) {
	cluster := "current"
	if args.Cluster != "" {
		cluster = args.Cluster
	}

	return mcpgolang.NewPromptResponse(
		"Debug instructions for a Flux Kustomization",
		mcpgolang.NewPromptMessage(
			mcpgolang.NewTextContent(fmt.Sprintf(`
To debug the Flux Kustomization %[1]s in namespace %[2]s on the %[3]s cluster, follow these steps:

0. Use the get_kubeconfig_contexts tool to find the context name for the %[3]s cluster
and use the set_kubeconfig_context to change the context to it.
1. Use the get_flux_instance_report tool to check the kustomize-controller deployment
status and the available Flux API versions.
2. Retrieve the Kustomization details using the get_kubernetes_resources tool.
3. Identify the Kustomization source by looking at the spec.sourceRef field,
then use the get_kubernetes_resources tool to fetch the corresponding
OCIRepository, Bucket, or GitRepository resource.
4. If the Kustomization is in a failed state or in progress, check the status
conditions and events for any error messages.
5. Use the get_kubernetes_resources tool to check the status of the resources
found in the Kustomization status inventory.
6. Write a detailed report of the issue, including the Kustomization spec, status, and any error messages.

`, args.Name, args.Namespace, cluster)),
			mcpgolang.RoleAssistant,
		),
	), nil
}
