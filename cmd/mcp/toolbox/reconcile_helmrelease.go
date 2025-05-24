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

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewReconcileHelmReleaseTool creates a new tool for reconciling a Flux HelmRelease.
func (m *Manager) NewReconcileHelmReleaseTool() SystemTool {
	return SystemTool{
		mcp.NewTool("reconcile_flux_helmrelease",
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
		m.HandleReconcileHelmRelease,
		false,
		true,
	}
}

// HandleReconcileHelmRelease is the handler function for the reconcile_flux_helmrelease tool.
func (m *Manager) HandleReconcileHelmRelease(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
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
			case "HelmChart":
				err = kubeClient.Annotate(ctx,
					schema.GroupVersionKind{
						Group:   "source.toolkit.fluxcd.io",
						Version: "v1",
						Kind:    "HelmChart",
					},
					chartRefName,
					chartRefNamespace,
					[]string{meta.ReconcileRequestAnnotation},
					ts)
			case "OCIRepository":
				err = kubeClient.Annotate(ctx,
					schema.GroupVersionKind{
						Group:   "source.toolkit.fluxcd.io",
						Version: "v1beta2",
						Kind:    "OCIRepository",
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
			Group:   "helm.toolkit.fluxcd.io",
			Version: "v2",
			Kind:    "HelmRelease",
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
