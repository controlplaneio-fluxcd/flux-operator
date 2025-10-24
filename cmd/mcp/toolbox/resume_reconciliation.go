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
	// ToolResumeFluxReconciliation is the name of the resume_flux_reconciliation tool.
	ToolResumeFluxReconciliation = "resume_flux_reconciliation"
)

func init() {
	systemTools[ToolResumeFluxReconciliation] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// resumeFluxReconciliationInput defines the input parameters for resuming the reconciliation of a Flux resource.
type resumeFluxReconciliationInput struct {
	APIVersion string `json:"apiVersion" jsonschema:"The apiVersion of the Flux resource."`
	Kind       string `json:"kind" jsonschema:"The kind of the Flux resource."`
	Name       string `json:"name" jsonschema:"The name of the Flux resource."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the Flux resource."`
}

// HandleResumeReconciliation is the handler function for the resume_flux_reconciliation tool.
func (m *Manager) HandleResumeReconciliation(ctx context.Context, request *mcp.CallToolRequest, input resumeFluxReconciliationInput) (*mcp.CallToolResult, any, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolResumeFluxReconciliation, m.readOnly)); err != nil {
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

	err = kubeClient.ToggleSuspension(ctx, gvk, input.Name, input.Namespace, false)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to resume reconciliation", err)
	}

	return NewToolResultText(fmt.Sprintf("Reconciliation of %s/%s/%s resumed and started", gvk.Kind, input.Namespace, input.Name))
}
