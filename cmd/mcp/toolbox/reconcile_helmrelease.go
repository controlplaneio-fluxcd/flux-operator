// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxHelmRelease is the name of the reconcile_flux_helmrelease tool.
	ToolReconcileFluxHelmRelease = "reconcile_flux_helmrelease"
)

// NewReconcileHelmReleaseTool creates a new tool for reconciling a Flux HelmRelease.
func (m *Manager) NewReconcileHelmReleaseTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool(ToolReconcileFluxHelmRelease,
			mcp.WithDescription("This tool triggers the reconciliation of a Flux HelmRelease  and optionally its source reference."),
			mcp.WithString("name",
				mcp.Description("The name of the HelmRelease."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the HelmRelease."),
				mcp.Required(),
			),
			mcp.WithBoolean("with_source",
				mcp.Description("If true, the source will be reconciled as well."),
			),
		),
		Handler:   m.HandleReconcileHelmRelease,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleReconcileHelmRelease is the handler function for the reconcile_flux_helmrelease tool.
func (m *Manager) HandleReconcileHelmRelease(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolReconcileFluxHelmRelease, m.readonly)); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	name := mcp.ParseString(request, "name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	namespace := mcp.ParseString(request, "namespace", "")
	if namespace == "" {
		return mcp.NewToolResultError("namespace is required"), nil
	}
	withSource := mcp.ParseBoolean(request, "with_source", false)

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(ctx, m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	hr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fluxcdv1.FluxHelmGroup + "/v2",
			"kind":       fluxcdv1.FluxHelmReleaseKind,
		},
	}
	hr.SetName(name)
	hr.SetNamespace(namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(hr), hr); err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to get HelmRelease", err), nil
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if withSource {
		chartRefType, found, err := unstructured.NestedString(hr.Object, "spec", "chartRef", "kind")
		if found && err == nil {
			chartRefName, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "name")
			chartRefNamespace, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "namespace")
			if chartRefNamespace == "" {
				chartRefNamespace = namespace
			}

			var err error
			switch chartRefType {
			case fluxcdv1.FluxHelmChartKind:
				err = kubeClient.Annotate(ctx,
					schema.GroupVersionKind{
						Group:   fluxcdv1.FluxSourceGroup,
						Version: "v1",
						Kind:    fluxcdv1.FluxHelmChartKind,
					},
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			case fluxcdv1.FluxOCIRepositoryKind:
				err = kubeClient.Annotate(ctx,
					schema.GroupVersionKind{
						Group:   fluxcdv1.FluxSourceGroup,
						Version: "v1beta2",
						Kind:    fluxcdv1.FluxOCIRepositoryKind,
					},
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			default:
				return mcp.NewToolResultError(fmt.Sprintf("Unknown chartRef kind %s", chartRefType)), nil
			}
			if err != nil {
				return mcp.NewToolResultErrorFromErr("Failed to reconcile source", err), nil
			}
		}
	}

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.FluxHelmGroup,
			Version: "v2",
			Kind:    fluxcdv1.FluxHelmReleaseKind,
		},
		name,
		namespace,
		[]string{
			meta.ReconcileRequestAnnotation,
			"reconcile.fluxcd.io/forceAt",
		},
		ts)

	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to reconcile HelmRelease", err), nil
	}

	return mcp.NewToolResultText(`HelmRelease reconciliation triggered.
To verify check the status lastHandledForceAt field matches the forceAt annotation.`), nil
}
