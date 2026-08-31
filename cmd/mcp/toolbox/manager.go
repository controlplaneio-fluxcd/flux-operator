// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// systemTool defines the common settings for MCP tools.
// All tools should register the properties on init() functions
// so RegisterTools can register them on the MCP server using
// the properties defined in this struct.
type systemTool struct {
	readOnly  bool
	inCluster bool
}

var (
	systemTools = map[string]systemTool{}
)

// Manager manages Kubernetes configurations and operations,
// providing MCP tools for context handling and resource management.
type Manager struct {
	kubeconfig  *k8s.KubeConfig
	kubeClient  *k8s.ClientFactory
	timeout     time.Duration
	maskSecrets bool
	readOnly    bool
	localFiles  bool
}

// NewManager initializes and returns a new Manager instance
// with the provided configuration and settings.
func NewManager(kubeClient *k8s.ClientFactory, timeout time.Duration,
	maskSecrets bool, readOnly bool, localFiles bool) *Manager {

	return &Manager{
		kubeconfig:  k8s.NewKubeConfig(),
		kubeClient:  kubeClient,
		timeout:     timeout,
		maskSecrets: maskSecrets,
		readOnly:    readOnly,
		localFiles:  localFiles,
	}
}

// toolRecorder records the tools added to the MCP server.
type toolRecorder struct {
	tools []string
}

// addTool adds a tool to the MCP server and records it.
// InputSchema is inferred before registration so it can be normalized before
// the server stores its copy of the tool.
func addTool[In, Out any](s *mcp.Server, r *toolRecorder, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
	if t.InputSchema == nil {
		inputType := reflect.TypeFor[In]()
		if inputType == reflect.TypeFor[any]() {
			t.InputSchema = &jsonschema.Schema{Type: "object"}
		} else {
			schema, err := jsonschema.ForType(inputType, &jsonschema.ForOptions{})
			if err != nil {
				panic(fmt.Sprintf("AddTool: tool %q: input schema: %v", t.Name, err))
			}
			t.InputSchema = schema
		}
	}
	if schema, ok := t.InputSchema.(*jsonschema.Schema); ok && schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	mcp.AddTool(s, t, h)
	r.tools = append(r.tools, t.Name)
}

// RegisterTools registers tools with the given server and returns the list of registered tool names.
func (m *Manager) RegisterTools(server *mcp.Server, inCluster bool) []string {
	var recorder toolRecorder
	if m.shouldRegisterTool(ToolInstallFluxInstance, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolInstallFluxInstance,
				Description: "Installs Flux Operator and a Flux instance on the cluster from a manifest URL.",
			},
			m.HandleInstallFluxInstance,
		)
	}
	if m.shouldRegisterTool(ToolGetFluxInstance, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetFluxInstance,
				Description: "Retrieves the Flux installation report with controllers, CRDs and their reconciliation status.",
			},
			m.HandleGetFluxInstance,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesAPIVersions, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesAPIVersions,
				Description: "Retrieves the CRDs registered on the cluster and their preferred apiVersion for each kind.",
			},
			m.HandleGetAPIVersions,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesLogs, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesLogs,
				Description: "Retrieves logs from a Kubernetes pod.",
			},
			m.HandleGetKubernetesLogs,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesEvents, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesEvents,
				Description: "Retrieves Kubernetes events, optionally filtered by namespace, involved object, type, time window and regex.",
			},
			m.HandleGetKubernetesEvents,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesMetrics, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesMetrics,
				Description: "Retrieves CPU and memory usage of pods.",
			},
			m.HandleGetKubernetesMetrics,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesResources, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesResources,
				Description: "Retrieves Kubernetes resources from the cluster.",
			},
			m.HandleGetKubernetesResources,
		)
	}
	if m.shouldRegisterTool(ToolSearchFluxDocs, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSearchFluxDocs,
				Description: "Searches the Flux Operator and Flux CRD documentation, returning the most relevant sections with their path and line range. Query with 2-5 keywords naming the kind and field (e.g. HelmRelease valuesFrom, CEL expression), not a question. Pages are published at https://fluxoperator.dev{path}/.",
			},
			m.HandleSearchFluxDocs,
		)
	}
	if m.shouldRegisterTool(ToolReadFluxDoc, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReadFluxDoc,
				Description: "Reads a Flux Operator documentation page as markdown. Use the path from search_flux_docs results. Long pages should be read in slices: pass heading to jump to a section, or offset and limit to page; the response reports the next offset. Output is capped at 30KB per call.",
			},
			m.HandleReadFluxDoc,
		)
	}
	if m.shouldRegisterTool(ToolDiffKubernetesManifest, inCluster) {
		tool := &mcp.Tool{
			Name:        ToolDiffKubernetesManifest,
			Description: "Diffs a Kubernetes YAML manifest against the cluster using a server-side apply dry-run.",
		}
		if !m.localFiles {
			inputType := reflect.TypeFor[diffKubernetesManifestInput]()
			schema, err := jsonschema.ForType(inputType, &jsonschema.ForOptions{})
			if err != nil {
				panic(fmt.Sprintf("AddTool: tool %q: input schema: %v", tool.Name, err))
			}
			delete(schema.Properties, "yaml_path")
			schema.Required = slices.DeleteFunc(schema.Required, func(field string) bool {
				return field == "yaml_path"
			})
			tool.InputSchema = schema
		}
		addTool(server, &recorder, tool, m.HandleDiffKubernetesManifest)
	}
	if m.shouldRegisterTool(ToolApplyKubernetesManifest, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolApplyKubernetesManifest,
				Description: "Applies a Kubernetes YAML manifest on the cluster.",
			},
			m.HandleApplyKubernetesManifest,
		)
	}
	if m.shouldRegisterTool(ToolPatchKubernetesResource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolPatchKubernetesResource,
				Description: "Patches a Kubernetes resource on the cluster with a merge, JSON or strategic merge patch.",
			},
			m.HandlePatchKubernetesResource,
		)
	}
	if m.shouldRegisterTool(ToolDeleteKubernetesResource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolDeleteKubernetesResource,
				Description: "Deletes a Kubernetes resource from the cluster.",
			},
			m.HandleDeleteKubernetesResource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxResource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxResource,
				Description: "Reconciles a Flux resource by triggering an on-demand reconciliation, optionally reconciling its source first.",
			},
			m.HandleReconcileResource,
		)
	}
	if m.shouldRegisterTool(ToolSuspendFluxReconciliation, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSuspendFluxReconciliation,
				Description: "Suspends reconciliation for a Flux resource.",
			},
			m.HandleSuspendReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolResumeFluxReconciliation, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolResumeFluxReconciliation,
				Description: "Resumes reconciliation for a Flux resource.",
			},
			m.HandleResumeReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolGetKubeConfigContexts, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubeConfigContexts,
				Description: "Retrieves the cluster names and contexts found in the kubeconfig.",
			},
			m.HandleGetKubeconfigContexts,
		)
	}
	if m.shouldRegisterTool(ToolSetKubeConfigContext, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSetKubeConfigContext,
				Description: "Sets the current kubeconfig context.",
			},
			m.HandleSetKubeconfigContext,
		)
	}
	return recorder.tools
}

// shouldRegisterTool checks if the tool is registered in all the global maps
// and if it should be registered based on the Manager settings and environment.
func (m *Manager) shouldRegisterTool(tool string, inCluster bool) bool {
	// Ensure tool has systemTools entry.
	t, ok := systemTools[tool]
	if !ok {
		panic(fmt.Sprintf("tool %s not registered in systemTools", tool))
	}

	// Ensure tool has also scopesPerTool entry.
	if _, ok := scopesPerTool[tool]; !ok {
		panic(fmt.Sprintf("tool %s not registered in scopesPerTool", tool))
	}

	// Check if should register tool.
	if inCluster && !t.inCluster {
		return false
	}
	if m.readOnly && !t.readOnly {
		return false
	}
	return true
}
