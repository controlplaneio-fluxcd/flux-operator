// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/auth"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolReconcileFluxKustomization is the name of the reconcile_flux_kustomization tool.
	ToolReconcileFluxKustomization = "reconcile_flux_kustomization"
)

func init() {
	systemTools[ToolReconcileFluxKustomization] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxKustomizationInput defines the input parameters for reconciling a Flux Kustomization.
type reconcileFluxKustomizationInput struct {
	Name       string `json:"name" jsonschema:"The name of the Flux Kustomization."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the Flux Kustomization."`
	WithSource bool   `json:"with_source,omitempty" jsonschema:"If true the source will be reconciled as well."`
}

// HandleReconcileKustomization is the handler function for the reconcile_flux_kustomization tool.
func (m *Manager) HandleReconcileKustomization(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxKustomizationInput) (*mcp.CallToolResult, any, error) {
	if err := auth.CheckScopes(ctx, getScopeNames(ToolReconcileFluxKustomization, m.readOnly)); err != nil {
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
	ks := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fluxcdv1.FluxKustomizeGroup + "/v1",
			"kind":       fluxcdv1.FluxKustomizationKind,
		},
	}
	ks.SetName(input.Name)
	ks.SetNamespace(input.Namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(ks), ks); err != nil {
		return NewToolResultErrorFromErr("Failed to get Kustomization", err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if input.WithSource {
		sourceRefType, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "kind")
		sourceRefName, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
		sourceRefNamespace, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "namespace")
		if sourceRefNamespace == "" {
			sourceRefNamespace = input.Namespace
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
			return NewToolResultError(fmt.Sprintf("Unknown sourceRef kind %s", sourceRefType))
		}
		if err != nil {
			return NewToolResultErrorFromErr("Failed to reconcile source", err)
		}
	}

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.FluxKustomizeGroup,
			Version: "v1",
			Kind:    fluxcdv1.FluxKustomizationKind,
		},
		input.Name,
		input.Namespace,
		[]string{meta.ReconcileRequestAnnotation},
		ts)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to reconcile Kustomization", err)
	}

	return NewToolResultText(`Kustomization reconciliation triggered.
To verify check the status lastHandledReconcileAt field matches the requestedAt annotation`)
}
