// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

type ReconcileTool struct {
	Name        string
	Description string
	Handler     any
}

var ReconcileToolList = []ReconcileTool{
	{
		Name:        "reconcile_flux_resourceset",
		Description: "This tool triggers the reconciliation of a Flux ResourceSet identified by name and namespace.",
		Handler:     ReconcileResourceSetHandler,
	},
	{
		Name:        "reconcile_flux_source",
		Description: "This tool triggers the reconciliation of a Flux source identified by kind, name and namespace.",
		Handler:     ReconcileSourceHandler,
	},
	{
		Name:        "reconcile_flux_kustomization",
		Description: "This tool triggers the reconciliation of a Flux Kustomization identified by name and namespace.",
		Handler:     ReconcileKustomizationHandler,
	},
	{
		Name:        "reconcile_flux_helmrelease",
		Description: "This tool triggers the reconciliation of a Flux HelmRelease identified by name and namespace.",
		Handler:     ReconcileHelmReleaseHandler,
	},
}
