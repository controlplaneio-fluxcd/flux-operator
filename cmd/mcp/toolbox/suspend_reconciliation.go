// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolSuspendFluxReconciliation is the name of the suspend_flux_reconciliation tool.
	ToolSuspendFluxReconciliation = "suspend_flux_reconciliation"
)

func init() {
	systemTools[ToolSuspendFluxReconciliation] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// suspendFluxReconciliationInput defines the input parameters for suspending the reconciliation of a Flux resource.
type suspendFluxReconciliationInput struct {
	APIVersion string `json:"apiVersion" jsonschema:"The apiVersion of the Flux resource."`
	Kind       string `json:"kind" jsonschema:"The kind of the Flux resource."`
	Name       string `json:"name" jsonschema:"The name of the Flux resource."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the Flux resource."`
}

// HandleSuspendReconciliation is the handler function for the suspend_flux_reconciliation tool.
func (m *Manager) HandleSuspendReconciliation(ctx context.Context, request *mcp.CallToolRequest, input suspendFluxReconciliationInput) (*mcp.CallToolResult, any, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolSuspendFluxReconciliation, m.readOnly)); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.APIVersion == "" {
		return NewToolResultError("apiVersion is required")
	}
	if input.Kind == "" {
		return NewToolResultError("kind is required")
	}
	if input.Name == "" {
		return NewToolResultError("name is required")
	}
	if input.Namespace == "" {
		return NewToolResultError("namespace is required")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to create Kubernetes client", err)
	}

	gvk, err := kubeClient.ParseGroupVersionKind(input.APIVersion, input.Kind)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to parse group version kind", err)
	}

	err = kubeClient.ToggleSuspension(ctx, gvk, input.Name, input.Namespace, true)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to suspend reconciliation", err)
	}

	return NewToolResultText(fmt.Sprintf("Reconciliation of %s/%s/%s suspended", gvk.Kind, input.Namespace, input.Name))
}
