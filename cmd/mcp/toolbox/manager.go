// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

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
	flags       *cli.ConfigFlags
	timeout     time.Duration
	maskSecrets bool
	readOnly    bool
}

// NewManager initializes and returns a new Manager instance
// with the provided configuration and settings.
func NewManager(flags *cli.ConfigFlags, timeout time.Duration, maskSecrets bool, readOnly bool) *Manager {
	m := &Manager{
		kubeconfig:  k8s.NewKubeConfig(),
		flags:       flags,
		timeout:     timeout,
		maskSecrets: maskSecrets,
		readOnly:    readOnly,
	}

	return m
}

// RegisterTools registers tools with the given server.
func (m *Manager) RegisterTools(server *mcp.Server, inCluster bool) {
	if m.shouldRegisterTool(ToolGetFluxInstance, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetFluxInstance,
				Description: "This tool retrieves the Flux instance installation and a detailed report about Flux controllers, CRDs and their status.",
			},
			m.HandleGetFluxInstance,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesAPIVersions, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetKubernetesAPIVersions,
				Description: "This tool retrieves the Kubernetes CRDs registered on the cluster and returns the preferred apiVersion for each kind.",
			},
			m.HandleGetAPIVersions,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesLogs, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetKubernetesLogs,
				Description: "This tool retrieves logs from a Kubernetes pod.",
			},
			m.HandleGetKubernetesLogs,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesMetrics, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetKubernetesMetrics,
				Description: "This tool retrieves metrics from a Kubernetes pod.",
			},
			m.HandleGetKubernetesMetrics,
		)
	}
	if m.shouldRegisterTool(ToolGetKubernetesResources, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetKubernetesResources,
				Description: "This tool retrieves Kubernetes resources from the cluster.",
			},
			m.HandleGetKubernetesResources,
		)
	}
	if m.shouldRegisterTool(ToolSearchFluxDocs, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolSearchFluxDocs,
				Description: "This tool searches the Flux documentation for a given query.",
			},
			m.HandleSearchFluxDocs,
		)
	}
	if m.shouldRegisterTool(ToolApplyKubernetesManifest, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolApplyKubernetesManifest,
				Description: "This tool applies a Kubernetes YAML manifest on the cluster.",
			},
			m.HandleApplyKubernetesManifest,
		)
	}
	if m.shouldRegisterTool(ToolDeleteKubernetesResource, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolDeleteKubernetesResource,
				Description: "This tool deletes a Kubernetes resource from the cluster.",
			},
			m.HandleDeleteKubernetesResource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxSource, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolReconcileFluxSource,
				Description: "This tool reconciles a Flux Source.",
			},
			m.HandleReconcileSource,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxKustomization, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolReconcileFluxKustomization,
				Description: "This tool reconciles a Flux Kustomization.",
			},
			m.HandleReconcileKustomization,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxHelmRelease, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolReconcileFluxHelmRelease,
				Description: "This tool reconciles a Flux HelmRelease.",
			},
			m.HandleReconcileHelmRelease,
		)
	}
	if m.shouldRegisterTool(ToolReconcileFluxResourceSet, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolReconcileFluxResourceSet,
				Description: "This tool reconciles a Flux ResourceSet.",
			},
			m.HandleReconcileResourceSet,
		)
	}
	if m.shouldRegisterTool(ToolSuspendFluxReconciliation, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolSuspendFluxReconciliation,
				Description: "This tool suspends reconciliation for a Flux resource.",
			},
			m.HandleSuspendReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolResumeFluxReconciliation, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolResumeFluxReconciliation,
				Description: "This tool resumes reconciliation for a Flux resource.",
			},
			m.HandleResumeReconciliation,
		)
	}
	if m.shouldRegisterTool(ToolGetKubeConfigContexts, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolGetKubeConfigContexts,
				Description: "This tool retrieves the Kubernetes clusters name and context found in the kubeconfig.",
			},
			m.HandleGetKubeconfigContexts,
		)
	}
	if m.shouldRegisterTool(ToolSetKubeConfigContext, inCluster) {
		mcp.AddTool(server,
			&mcp.Tool{
				Name:        ToolSetKubeConfigContext,
				Description: "This tool sets the current Kubernetes context in the kubeconfig.",
			},
			m.HandleSetKubeconfigContext,
		)
	}
}

func (m *Manager) shouldRegisterTool(tool string, inCluster bool) bool {
	if t, ok := systemTools[tool]; ok {
		if inCluster && !t.inCluster {
			return false
		}
		if m.readOnly && !t.readOnly {
			return false
		}
		return true
	}
	panic(fmt.Sprintf("tool %s not registered in systemTools", tool))
}
