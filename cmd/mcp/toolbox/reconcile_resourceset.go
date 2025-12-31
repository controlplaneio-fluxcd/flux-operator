// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxResourceSet is the name of the reconcile_flux_resourceset tool.
	ToolReconcileFluxResourceSet = "reconcile_flux_resourceset"
)

func init() {
	systemTools[ToolReconcileFluxResourceSet] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxResourceSetInput defines the input parameters for reconciling a Flux ResourceSet.
type reconcileFluxResourceSetInput struct {
	Name      string `json:"name" jsonschema:"The name of the ResourceSet."`
	Namespace string `json:"namespace" jsonschema:"The namespace of the ResourceSet."`
}

// HandleReconcileResourceSet is the handler function for the reconcile_flux_resourceset tool.
func (m *Manager) HandleReconcileResourceSet(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxResourceSetInput) (*mcp.CallToolResult, any, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolReconcileFluxResourceSet, m.readOnly)); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.Name == "" {
		return NewToolResultError("name is required")
	}
	if input.Namespace == "" {
		return NewToolResultError("namespace is required")
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags, m.kubeconfig.CurrentContextName)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to create Kubernetes client", err)
	}

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.GroupVersion.Group,
			Version: fluxcdv1.GroupVersion.Version,
			Kind:    fluxcdv1.ResourceSetKind,
		},
		input.Name,
		input.Namespace,
		[]string{meta.ReconcileRequestAnnotation},
		time.Now().Format(time.RFC3339Nano))

	if err != nil {
		return NewToolResultErrorFromErr("Unable to reconcile ResourceSet", err)
	}

	return NewToolResultText(`ResourceSet reconciliation triggered.
to verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)
}
