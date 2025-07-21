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
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewReconcileKustomizationTool creates a new tool for reconciling a Flux Kustomization.
func (m *Manager) NewReconcileKustomizationTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("reconcile_flux_kustomization",
			mcp.WithDescription("This tool triggers the reconciliation of a Flux Kustomization and optionally its source reference."),
			mcp.WithString("name",
				mcp.Description("The name of the Flux Kustomization."),
				mcp.Required(),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the Flux Kustomization."),
				mcp.Required(),
			),
			mcp.WithBoolean("with_source",
				mcp.Description("If true, the source will be reconciled as well."),
			),
		),
		Handler:   m.HandleReconcileKustomization,
		ReadOnly:  false,
		InCluster: true,
	}
}

// HandleReconcileKustomization is the handler function for the reconcile_flux_kustomization tool.
func (m *Manager) HandleReconcileKustomization(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	ks := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fluxcdv1.FluxKustomizeGroup + "/v1",
			"kind":       fluxcdv1.FluxKustomizationKind,
		},
	}
	ks.SetName(name)
	ks.SetNamespace(namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(ks), ks); err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to get Kustomization", err), nil
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if withSource {
		sourceRefType, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "kind")
		sourceRefName, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
		sourceRefNamespace, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "namespace")
		if sourceRefNamespace == "" {
			sourceRefNamespace = namespace
		}

		var err error
		switch sourceRefType {
		case fluxcdv1.FluxGitRepositoryKind:
			err = kubeClient.Annotate(ctx,
				schema.GroupVersionKind{
					Group:   fluxcdv1.FluxSourceGroup,
					Version: "v1",
					Kind:    fluxcdv1.FluxGitRepositoryKind,
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case fluxcdv1.FluxBucketKind:
			err = kubeClient.Annotate(ctx,
				schema.GroupVersionKind{
					Group:   fluxcdv1.FluxSourceGroup,
					Version: "v1",
					Kind:    fluxcdv1.FluxBucketKind,
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		case fluxcdv1.FluxOCIRepositoryKind:
			err = kubeClient.Annotate(ctx,
				schema.GroupVersionKind{
					Group:   fluxcdv1.FluxSourceGroup,
					Version: "v1beta2",
					Kind:    fluxcdv1.FluxOCIRepositoryKind,
				},
				sourceRefName,
				sourceRefNamespace,
				[]string{meta.ReconcileRequestAnnotation},
				ts)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown sourceRef kind %s", sourceRefType)), nil
		}
		if err != nil {
			return mcp.NewToolResultErrorFromErr("Failed to reconcile source", err), nil
		}
	}

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.FluxKustomizeGroup,
			Version: "v1",
			Kind:    fluxcdv1.FluxKustomizationKind,
		},
		name,
		namespace,
		[]string{meta.ReconcileRequestAnnotation},
		ts)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to reconcile Kustomization", err), nil
	}

	return mcp.NewToolResultText(`Kustomization reconciliation triggered.
To verify check the status lastHandledReconcileAt field matches the requestedAt annotation`), nil
}
