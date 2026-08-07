// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"encoding/json"
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
		"reconcile_flux_chain",
	}))
}

func TestManager_ToolSchemasIncludeProperties(t *testing.T) {
	g := NewWithT(t)

	expectedSchemas := map[string]struct {
		properties []string
		required   []string
	}{
		ToolInstallFluxInstance: {
			properties: []string{"instance_url", "timeout"},
			required:   []string{"instance_url"},
		},
		ToolGetFluxInstance: {
			properties: []string{},
			required:   []string{},
		},
		ToolGetKubernetesAPIVersions: {
			properties: []string{},
			required:   []string{},
		},
		ToolGetKubernetesLogs: {
			properties: []string{"pod_name", "container_name", "pod_namespace", "limit", "previous"},
			required:   []string{"pod_name", "container_name", "pod_namespace"},
		},
		ToolGetKubernetesMetrics: {
			properties: []string{"pod_name", "pod_namespace", "pod_selector", "limit"},
			required:   []string{"pod_namespace"},
		},
		ToolGetKubernetesResources: {
			properties: []string{"apiVersion", "kind", "name", "namespace", "selector", "limit"},
			required:   []string{"apiVersion", "kind"},
		},
		ToolSearchFluxDocs: {
			properties: []string{"query", "limit", "format"},
			required:   []string{"query"},
		},
		ToolApplyKubernetesManifest: {
			properties: []string{"yaml_content", "overwrite"},
			required:   []string{"yaml_content"},
		},
		ToolDeleteKubernetesResource: {
			properties: []string{"apiVersion", "kind", "name", "namespace"},
			required:   []string{"apiVersion", "kind", "name"},
		},
		ToolReconcileFluxSource: {
			properties: []string{"kind", "name", "namespace"},
			required:   []string{"kind", "name", "namespace"},
		},
		ToolReconcileFluxKustomization: {
			properties: []string{"name", "namespace", "with_source"},
			required:   []string{"name", "namespace"},
		},
		ToolReconcileFluxHelmRelease: {
			properties: []string{"name", "namespace", "with_source"},
			required:   []string{"name", "namespace"},
		},
		ToolReconcileFluxResourceSet: {
			properties: []string{"name", "namespace"},
			required:   []string{"name", "namespace"},
		},
		ToolSuspendFluxReconciliation: {
			properties: []string{"apiVersion", "kind", "name", "namespace"},
			required:   []string{"apiVersion", "kind", "name", "namespace"},
		},
		ToolResumeFluxReconciliation: {
			properties: []string{"apiVersion", "kind", "name", "namespace"},
			required:   []string{"apiVersion", "kind", "name", "namespace"},
		},
		ToolGetKubeConfigContexts: {
			properties: []string{},
			required:   []string{},
		},
		ToolSetKubeConfigContext: {
			properties: []string{"name"},
			required:   []string{"name"},
		},
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "flux-operator-mcp",
		Version: "test-version",
	}, &mcp.ServerOptions{
		HasTools: true,
	})

	manager := NewManager(nil, 0, false, false, nil)
	manager.RegisterTools(server, false)

	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	_, err := server.Connect(ctx, st, nil)
	g.Expect(err).NotTo(HaveOccurred())

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "test-version",
	}, nil)
	session, err := client.Connect(ctx, ct, nil)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Tools).To(HaveLen(len(expectedSchemas)))

	for _, tool := range result.Tools {
		expectedSchema, ok := expectedSchemas[tool.Name]
		g.Expect(ok).To(BeTrue(), "unexpected tool %s", tool.Name)

		raw, err := json.Marshal(tool.InputSchema)
		g.Expect(err).NotTo(HaveOccurred(), "failed to marshal schema for tool %s", tool.Name)

		var schema map[string]any
		err = json.Unmarshal(raw, &schema)
		g.Expect(err).NotTo(HaveOccurred(), "failed to unmarshal schema for tool %s", tool.Name)

		g.Expect(schema).To(HaveKey("properties"),
			"tool %s schema is missing 'properties' field (required by OpenAI function calling API): %s",
			tool.Name, string(raw))

		properties, ok := schema["properties"].(map[string]any)
		g.Expect(ok).To(BeTrue(), "tool %s schema has invalid properties field: %s", tool.Name, string(raw))
		g.Expect(properties).To(HaveLen(len(expectedSchema.properties)),
			"tool %s schema has unexpected properties: %s", tool.Name, string(raw))
		for _, property := range expectedSchema.properties {
			g.Expect(properties).To(HaveKey(property),
				"tool %s schema is missing inferred %s property: %s",
				tool.Name, property, string(raw))
		}

		var required []string
		if rawRequired, ok := schema["required"]; ok {
			requiredFields, ok := rawRequired.([]any)
			g.Expect(ok).To(BeTrue(), "tool %s schema has invalid required field: %s", tool.Name, string(raw))
			for _, field := range requiredFields {
				fieldName, ok := field.(string)
				g.Expect(ok).To(BeTrue(), "tool %s schema has non-string required field: %s", tool.Name, string(raw))
				required = append(required, fieldName)
			}
		}
		g.Expect(required).To(ConsistOf(expectedSchema.required),
			"tool %s schema has unexpected required fields: %s",
			tool.Name, string(raw))
	}
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
