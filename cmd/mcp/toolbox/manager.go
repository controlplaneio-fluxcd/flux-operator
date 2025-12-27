// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"fmt"
	"slices"
	"time"

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
	enabled     []string
}

// NewManager initializes and returns a new Manager instance
// with the provided configuration and settings.
func NewManager(kubeClient *k8s.ClientFactory, timeout time.Duration,
	maskSecrets bool, readOnly bool, enabled []string) *Manager {

	return &Manager{
		kubeconfig:  k8s.NewKubeConfig(),
		kubeClient:  kubeClient,
		timeout:     timeout,
		maskSecrets: maskSecrets,
		readOnly:    readOnly,
		enabled:     enabled,
	}
}

// toolRecorder records the tools added to the MCP server.
type toolRecorder struct {
	tools []string
}

// addTool adds a tool to the MCP server and records it.
func addTool[In, Out any](s *mcp.Server, r *toolRecorder, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) {
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
				Description: "This tool installs Flux Operator and a Flux instance on the cluster from a manifest URL.",
			},
			m.HandleInstallFluxInstance,
		)
	}
	if m.shouldRegisterTool(ToolGetFluxInstance, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetFluxInstance,
				Description: "This tool retrieves the Flux instance installation and a detailed report about Flux controllers, CRDs and their status.",
			},
			m.HandleGetFluxInstance,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesAPIVersions, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesAPIVersions,
				Description: "This tool retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind.",
			},
			m.HandleGetAPIVersions,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesLogs, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesLogs,
				Description: "This tool retrieves logs from a Kubernetes pod.",
			},
			m.HandleGetKubernetesLogs,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesMetrics, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesMetrics,
				Description: "This tool retrieves metrics from a Kubernetes pod.",
			},
			m.HandleGetKubernetesMetrics,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesResources, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubernetesResources,
				Description: "This tool retrieves Kubernetes resources from the cluster.",
			},
			m.HandleGetKubernetesResources,
		)
	}
	if m.shouldRegisterTool(ToolSearchFluxDocs, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSearchFluxDocs,
				Description: "This tool searches the Flux documentation for a given query.",
			},
			m.HandleSearchFluxDocs,
		)
	}
	if m.shouldRegisterTool(ToolApplyKubernetesManifest, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolApplyKubernetesManifest,
				Description: "This tool applies a Kubernetes YAML manifest on the cluster.",
			},
			m.HandleApplyKubernetesManifest,
		)
	}
	if m.shouldRegisterTool(ToolDeleteKubernetesResource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolDeleteKubernetesResource,
				Description: "This tool deletes a Kubernetes resource from the cluster.",
			},
			m.HandleDeleteKubernetesResource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxSource, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxSource,
				Description: "This tool reconciles a Flux Source.",
			},
			m.HandleReconcileSource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxKustomization, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxKustomization,
				Description: "This tool reconciles a Flux Kustomization.",
			},
			m.HandleReconcileKustomization,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxHelmRelease, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxHelmRelease,
				Description: "This tool reconciles a Flux HelmRelease.",
			},
			m.HandleReconcileHelmRelease,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxResourceSet, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolReconcileFluxResourceSet,
				Description: "This tool reconciles a Flux ResourceSet.",
			},
			m.HandleReconcileResourceSet,
		)
	}
	if m.shouldRegisterTool(ToolSuspendFluxReconciliation, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSuspendFluxReconciliation,
				Description: "This tool suspends reconciliation for a Flux resource.",
			},
			m.HandleSuspendReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolResumeFluxReconciliation, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolResumeFluxReconciliation,
				Description: "This tool resumes reconciliation for a Flux resource.",
			},
			m.HandleResumeReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolGetKubeConfigContexts, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolGetKubeConfigContexts,
				Description: "This tool retrieves the Kubernetes clusters name and context found in the kubeconfig.",
			},
			m.HandleGetKubeconfigContexts,
		)
	}
	if m.shouldRegisterTool(ToolSetKubeConfigContext, inCluster) {
		addTool(server, &recorder,
			&mcp.Tool{
				Name:        ToolSetKubeConfigContext,
				Description: "This tool sets the current Kubernetes context in the kubeconfig.",
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
	if len(m.enabled) > 0 && !slices.Contains(m.enabled, tool) {
		return false
	}
	if inCluster && !t.inCluster {
		return false
	}
	if m.readOnly && !t.readOnly {
		return false
	}
	return true
}
