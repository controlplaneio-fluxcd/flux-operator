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
				Description: "Searches the Flux documentation, returning matching sections with titles, links and excerpts. Concise by default, complete API docs on request.",
			},
			m.HandleSearchFluxDocs,
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
	if m.shouldRegisterTool(ToolDeleteKubernetesResource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolDeleteKubernetesResource,
				Description: "Deletes a Kubernetes resource from the cluster.",
			},
			m.HandleDeleteKubernetesResource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxSource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxSource,
				Description: "Reconciles a Flux Source.",
			},
			m.HandleReconcileSource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxKustomization, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxKustomization,
				Description: "Reconciles a Flux Kustomization.",
			},
			m.HandleReconcileKustomization,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxHelmRelease, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxHelmRelease,
				Description: "Reconciles a Flux HelmRelease.",
			},
			m.HandleReconcileHelmRelease,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxResourceSet, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxResourceSet,
				Description: "Reconciles a Flux ResourceSet.",
			},
			m.HandleReconcileResourceSet,
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
