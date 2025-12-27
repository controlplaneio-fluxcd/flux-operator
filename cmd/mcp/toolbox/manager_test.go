// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/gomega"
)

func TestManager_RegisterToolsDoesNotPanic(t *testing.T) {
	g := NewWithT(t)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "flux-operator-mcp",
		Version: "test-version",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	manager := NewManager(nil, 0, false, false, nil)
	registeredTools := manager.RegisterTools(server, false)
	g.Expect(registeredTools).To(Equal([]string{
		"install_flux_instance",
		"get_flux_instance",
		"get_kubernetes_api_versions",
		"get_kubernetes_logs",
		"get_kubernetes_metrics",
		"get_kubernetes_resources",
		"search_flux_docs",
		"apply_kubernetes_manifest",
		"delete_kubernetes_resource",
		"reconcile_flux_source",
		"reconcile_flux_kustomization",
		"reconcile_flux_helmrelease",
		"reconcile_flux_resourceset",
		"suspend_flux_reconciliation",
		"resume_flux_reconciliation",
		"get_kubeconfig_contexts",
		"set_kubeconfig_context",
	}))
}

func TestManager_RegisterSpecificTools(t *testing.T) {
	g := NewWithT(t)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "flux-operator-mcp",
		Version: "test-version",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	manager := NewManager(nil, 0, false, false, []string{
		"get_kubeconfig_contexts",
		"set_kubeconfig_context",
	})
	registeredTools := manager.RegisterTools(server, false)
	g.Expect(registeredTools).To(Equal([]string{
		"get_kubeconfig_contexts",
		"set_kubeconfig_context",
	}))
}
