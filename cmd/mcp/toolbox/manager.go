// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	cli "k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// SystemTool represents a system tool with its associated
// MCP tool, handler function, and read-only status.
type SystemTool struct {
	Tool    mcp.Tool
	Handler mcpserver.ToolHandlerFunc

	// ReadOnly indicates whether the tool is read-only,
	// meaning it does not modify the state of cluster resources.
	ReadOnly bool

	// InCluster indicates whether the tool can operate
	// inside a Kubernetes cluster using the in-cluster configuration.
	InCluster bool
}

// Manager manages Kubernetes configurations and operations,
// providing MCP tools for context handling and resource management.
type Manager struct {
	kubeconfig  *k8s.KubeConfig
	flags       *cli.ConfigFlags
	timeout     time.Duration
	maskSecrets bool
}

// NewManager initializes and returns a new Manager instance
// with the provided configuration and settings.
func NewManager(flags *cli.ConfigFlags, timeout time.Duration, maskSecrets bool) *Manager {
	m := &Manager{
		kubeconfig:  k8s.NewKubeConfig(),
		flags:       flags,
		timeout:     timeout,
		maskSecrets: maskSecrets,
	}

	return m
}

// RegisterTools registers tools with the given server,
// optionally filtering by readonly status.
func (m *Manager) RegisterTools(server *mcpserver.MCPServer, readonly bool, inCluster bool) {
	for _, t := range m.ToolSet() {
		if readonly && !t.ReadOnly {
			continue
		}
		if inCluster && !t.InCluster {
			continue
		}
		server.AddTool(t.Tool, t.Handler)
	}
}
