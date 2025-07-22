// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewReconcileSourceTool creates a new tool for reconciling a Flux source.
func (m *Manager) NewReconcileSourceTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("reconcile_flux_source",
			mcp.WithDescription("This tool triggers the reconciliation of a Flux source."),
			mcp.WithString("kind",
				mcp.Description("The Flux source kind. Can only one of GitRepository, OCIRepository, Bucket, HelmChart, HelmRepository."),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("The name of the Flux object."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the Flux object."),
				mcp.Required(),
			),
		),
		Handler:   m.HandleReconcileSource,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleReconcileSource is the handler function for the reconcile_flux_source tool.
func (m *Manager) HandleReconcileSource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	kind := mcp.ParseString(request, "kind", "")
	if kind == "" {
		return mcp.NewToolResultError("kind is required"), nil
	}
	name := mcp.ParseString(request, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	namespace := mcp.ParseString(request, "namespace", "")
	if namespace == "" {
		return mcp.NewToolResultError("namespace is required"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	ts := time.Now().Format(time.RFC3339Nano)
	switch kind {
	case fluxcdv1.FluxGitRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxGitRepositoryKind,
			},
			name,
			namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxBucketKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxBucketKind,
			},
			name,
			namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxHelmChartKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxHelmChartKind,
			},
			name,
			namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxHelmRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1",
				Kind:    fluxcdv1.FluxHelmRepositoryKind,
			},
			name,
			namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	case fluxcdv1.FluxOCIRepositoryKind:
		err = kubeClient.Annotate(ctx,
			schema.GroupVersionKind{
				Group:   fluxcdv1.FluxSourceGroup,
				Version: "v1beta2",
				Kind:    fluxcdv1.FluxOCIRepositoryKind,
			},
			name,
			namespace,
			[]string{meta.ReconcileRequestAnnotation},
			ts)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unknown source kind %s", kind)), nil
	}
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to annotate source", err), nil
	}

	return mcp.NewToolResultText(`Source reconciliation triggered.
To verify check the status lastHandledReconcileAt field matches the requestedAt annotation`), nil
}
