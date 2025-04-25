// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

type DeleteTool struct {
	Name        string
	Description string
	Handler     any
}

var DeleteToolList = []DeleteTool{
	{
		Name:        "delete_kubernetes_resource",
		Description: "This tool deletes a Kubernetes resource identified by apiVersion, kind, name and namespace.",
		Handler:     DeleteKubernetesResourceHandler,
	},
}
