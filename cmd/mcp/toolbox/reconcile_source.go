// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxSource is the name of the reconcile_flux_source tool.
	ToolReconcileFluxSource = "reconcile_flux_source"
)

func init() {
	systemTools[ToolReconcileFluxSource] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxSourceInput defines the input parameters for reconciling a Flux source.
type reconcileFluxSourceInput struct {
	Kind      string `json:"kind" jsonschema:"The Flux source kind. Can only one of GitRepository OCIRepository Bucket HelmChart HelmRepository."`
	Name      string `json:"name" jsonschema:"The name of the Flux object."`
	Namespace string `json:"namespace" jsonschema:"The namespace of the Flux object."`
}

// HandleReconcileSource is the handler function for the reconcile_flux_source tool.
func (m *Manager) HandleReconcileSource(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxSourceInput) (*mcp.CallToolResult, any, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolReconcileFluxSource, m.readOnly)); err != nil {
		return NewToolResultError(err.Error())
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

	kubeClient, err := k8s.NewClient(ctx, m.flags, m.kubeconfig.CurrentContextName)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to create Kubernetes client", err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	switch input.Kind {
	case fluxcdv1.FluxGitRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxGitRepositoryKind,
			},
			input.Name,
			input.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxBucketKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxBucketKind,
			},
			input.Name,
			input.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxHelmChartKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxHelmChartKind,
			},
			input.Name,
			input.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxHelmRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxHelmRepositoryKind,
			},
			input.Name,
			input.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxOCIRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1beta2",
				Kind:    fluxcdv1.FluxOCIRepositoryKind,
			},
			input.Name,
			input.Namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	default:
		return NewToolResultError(fmt.Sprintf("Unknown source kind %s", input.Kind))
	}
	if err != nil {
		return NewToolResultErrorFromErr("Failed to annotate source", err)
	}

	return NewToolResultText(`Source reconciliation triggered.
To verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)
}
