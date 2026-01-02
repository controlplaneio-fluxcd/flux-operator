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
)

const (
	// ToolReconcileFluxHelmRelease is the name of the reconcile_flux_helmrelease tool.
	ToolReconcileFluxHelmRelease = "reconcile_flux_helmrelease"
)

func init() {
	systemTools[ToolReconcileFluxHelmRelease] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// reconcileFluxHelmReleaseInput defines the input parameters for reconciling a Flux HelmRelease.
type reconcileFluxHelmReleaseInput struct {
	Name       string `json:"name" jsonschema:"The name of the HelmRelease."`
	Namespace  string `json:"namespace" jsonschema:"The namespace of the HelmRelease."`
	WithSource bool   `json:"with_source,omitempty" jsonschema:"If true the source will be reconciled as well."`
}

// HandleReconcileHelmRelease is the handler function for the reconcile_flux_helmrelease tool.
func (m *Manager) HandleReconcileHelmRelease(ctx context.Context, request *mcp.CallToolRequest, input reconcileFluxHelmReleaseInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolReconcileFluxHelmRelease, m.readOnly); err != nil {
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

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	hr := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fluxcdv1.FluxHelmGroup + "/v2",
			"kind":       fluxcdv1.FluxHelmReleaseKind,
		},
	}
	hr.SetName(input.Name)
	hr.SetNamespace(input.Namespace)

	if err := kubeClient.Get(ctx, kubeClient.ObjectKeyFromObject(hr), hr); err != nil {
		return NewToolResultErrorFromErr("Failed to get HelmRelease", err)
	}

	ts := time.Now().Format(time.RFC3339Nano)
	if input.WithSource {
		chartRefType, found, err := unstructured.NestedString(hr.Object, "spec", "chartRef", "kind")
		if found && err == nil {
			chartRefName, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "name")
			chartRefNamespace, _, _ := unstructured.NestedString(hr.Object, "spec", "chartRef", "namespace")
			if chartRefNamespace == "" {
				chartRefNamespace = input.Namespace
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
				return NewToolResultError(fmt.Sprintf("Unknown chartRef kind %s", chartRefType))
			}
			if err != nil {
				return NewToolResultErrorFromErr("Failed to reconcile source", err)
			}
		}
	}

	err = kubeClient.Annotate(ctx,
		schema.GroupVersionKind{
			Group:   fluxcdv1.FluxHelmGroup,
			Version: "v2",
			Kind:    fluxcdv1.FluxHelmReleaseKind,
		},
		input.Name,
		input.Namespace,
		[]string{
			meta.ReconcileRequestAnnotation,
			"reconcile.fluxcd.io/forceAt",
		},
		ts)

	if err != nil {
		return NewToolResultErrorFromErr("Failed to reconcile HelmRelease", err)
	}

	return NewToolResultText(`HelmRelease reconciliation triggered.
To verify check the status lastHandledForceAt field matches the forceAt annotation.`)
}
