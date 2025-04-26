// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

type GetTool struct {
	Name        string
	Description string
	Handler     any
}

var GetToolList = []GetTool{
	{
		Name:        "get_flux_instance_report",
		Description: "This tool retrieves the Flux instance installation and a detailed report about Flux controllers, CRDs and their status.",
		Handler:     GetFluxInstanceHandler,
	},
	{
		Name:        "get_kubernetes_resources",
		Description: "This tool retrieves Kubernetes resources identified by apiVersion, kind, name, namespace and label selector.",
		Handler:     GetKubernetesResourcesHandler,
	},
	{
		Name:        "get_kubernetes_api_versions",
		Description: "This tool retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind.",
		Handler:     GetApiVersionsHandler,
	},
}
