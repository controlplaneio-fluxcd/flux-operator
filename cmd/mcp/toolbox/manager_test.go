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
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})

	manager := NewManager(nil, 0, false, false, false)
	registeredTools := manager.RegisterTools(server, false)
	g.Expect(registeredTools).To(Equal([]string{
		"install_flux_instance",
		"get_flux_instance",
		"get_kubernetes_api_versions",
		"get_kubernetes_logs",
		"get_kubernetes_metrics",
		"get_kubernetes_resources",
		"search_flux_docs",
		"diff_kubernetes_manifest",
		"apply_kubernetes_manifest",
		"delete_kubernetes_resource",
		"reconcile_flux_resource",
		"suspend_flux_reconciliation",
		"resume_flux_reconciliation",
		"get_kubeconfig_contexts",
		"set_kubeconfig_context",
	}))
}

func TestManager_RegisterDiffKubernetesManifestModes(t *testing.T) {
	tests := []struct {
		name      string
		readOnly  bool
		inCluster bool
	}{
		{name: "read-only", readOnly: true},
		{name: "in-cluster", inCluster: true},
		{name: "read-only in-cluster", readOnly: true, inCluster: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			server := mcp.NewServer(&mcp.Implementation{
				Name:    "flux-operator-mcp",
				Version: "test-version",
			}, nil)
			manager := NewManager(nil, 0, false, tt.readOnly, false)
			registeredTools := manager.RegisterTools(server, tt.inCluster)
			g.Expect(registeredTools).To(ContainElement(ToolDiffKubernetesManifest))
		})
	}
}

func TestManager_DiffKubernetesManifestSchemaLocalFiles(t *testing.T) {
	tests := []struct {
		name       string
		localFiles bool
		hasPath    bool
	}{
		{name: "local files disabled", localFiles: false, hasPath: false},
		{name: "local files enabled", localFiles: true, hasPath: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			server := mcp.NewServer(&mcp.Implementation{
				Name:    "flux-operator-mcp",
				Version: "test-version",
			}, &mcp.ServerOptions{
				Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
			})
			manager := NewManager(nil, 0, false, false, tt.localFiles)
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
			g.Expect(result.Tools).NotTo(BeEmpty())

			var diffTool *mcp.Tool
			for i := range result.Tools {
				if result.Tools[i].Name == ToolDiffKubernetesManifest {
					diffTool = result.Tools[i]
					break
				}
			}
			g.Expect(diffTool).NotTo(BeNil())

			raw, err := json.Marshal(diffTool.InputSchema)
			g.Expect(err).NotTo(HaveOccurred())
			var schema struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
				Required []string `json:"required"`
			}
			err = json.Unmarshal(raw, &schema)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(schema.Properties["yaml_content"].Description).NotTo(ContainSubstring("yaml_path"))
			g.Expect(schema.Required).NotTo(ContainElement("yaml_path"))

			yamlPath, hasPath := schema.Properties["yaml_path"]
			g.Expect(hasPath).To(Equal(tt.hasPath))
			if tt.hasPath {
				g.Expect(yamlPath.Description).To(Equal("Absolute path to a local multi-doc YAML file to diff."))
			}
		})
	}
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
			properties: []string{"apiVersion", "kind", "name", "namespace", "selector", "limit", "fields"},
			required:   []string{"apiVersion", "kind"},
		},
		ToolSearchFluxDocs: {
			properties: []string{"query", "limit", "format"},
			required:   []string{"query"},
		},
		ToolDiffKubernetesManifest: {
			properties: []string{"yaml_content", "flux_object", "owner_kind", "owner_name", "owner_namespace"},
			required:   []string{},
		},
		ToolApplyKubernetesManifest: {
			properties: []string{"yaml_content", "overwrite"},
			required:   []string{"yaml_content"},
		},
		ToolDeleteKubernetesResource: {
			properties: []string{"apiVersion", "kind", "name", "namespace"},
			required:   []string{"apiVersion", "kind", "name"},
		},
		ToolReconcileFluxResource: {
			properties: []string{"apiVersion", "kind", "name", "namespace", "with_source"},
			required:   []string{"apiVersion", "kind", "name", "namespace"},
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
		Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
	})

	manager := NewManager(nil, 0, false, false, false)
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
