// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

// NewGetKubernetesResourcesTool creates a new tool for retrieving Kubernetes resources.
func (m *Manager) NewGetKubernetesResourcesTool() SystemTool {
	return SystemTool{
		Tool: mcp.NewTool("get_kubernetes_resources",
			mcp.WithDescription("This tool retrieves Kubernetes resources including Flux own resources, their status, and events"),
			mcp.WithString("apiVersion",
				mcp.Description("The apiVersion of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available apiVersions."),
				mcp.Required(),
			),
			mcp.WithString("kind",
				mcp.Description("The kind of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available kinds."),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("The name of the Kubernetes resource."),
			),
			mcp.WithString("namespace",
				mcp.Description("The namespace of the Kubernetes resource."),
			),
			mcp.WithString("selector",
				mcp.Description("The label selector in the format key1=value1,key2=value2."),
			),
			mcp.WithNumber("limit",
				mcp.Description("The maximum number of resources to return."),
			),
		),
		Handler:   m.HandleGetKubernetesResources,
		ReadOnly:  true,
		InCluster: true,
	}
}

// HandleGetKubernetesResources is the handler function for the get_kubernetes_resources tool.
func (m *Manager) HandleGetKubernetesResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	apiVersion := mcp.ParseString(request, "apiVersion", "")
	if apiVersion == "" {
		return mcp.NewToolResultError("apiVersion is required"), nil
	}
	kind := mcp.ParseString(request, "kind", "")
	if kind == "" {
		return mcp.NewToolResultError("kind is required"), nil
	}
	name := mcp.ParseString(request, "name", "")
	namespace := mcp.ParseString(request, "namespace", "")
	selector := mcp.ParseString(request, "selector", "")
	limit := mcp.ParseInt(request, "limit", 0)

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := k8s.NewClient(m.flags)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to create Kubernetes client", err), nil
	}

	gvk, err := kubeClient.ParseGroupVersionKind(apiVersion, kind)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to parse group version kind", err), nil
	}

	result, err := kubeClient.Export(ctx,
		[]schema.GroupVersionKind{gvk},
		name,
		namespace,
		selector,
		limit,
		m.maskSecrets)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to export resources", err), nil
	}

	if result == "" {
		return mcp.NewToolResultError("No resources found"), nil
	}

	return mcp.NewToolResultText(result + m.GetFluxTip(kind)), nil
}

func (m *Manager) GetFluxTip(kind string) string {
	switch kind {
	case fluxcdv1.FluxGitRepositoryKind, fluxcdv1.FluxBucketKind, fluxcdv1.FluxOCIRepositoryKind:
		return `
If asked for recommendations:
1. Check if the interval is less than 1 minute and if so, recommend to increase it to one minute.
2. Check if the GitRepository has ref.branch set and if so, recommend to set ref.name to refs/heads/<branch name>.
3. Check if the GitRepository has ref.tag set and if so, recommend to set ref.name to refs/tags/<tag name>.
`
	case fluxcdv1.FluxKustomizationKind:
		return `
If asked for recommendations:
1. Check if the Kustomization interval is less than 10 minutes and if so, recommend to increase it.
   Explain that the Kustomization interval is for detecting drift in cluster and undo kubectl edits.
   The interval set in the source (GitRepository, OCIRepository or Bucket) of the Kustomization
   is for detecting changes in upstream, and that one can be set to a lower value.
2. Check if the Kustomization has a retryInterval and if not, recommend to add it.
3. Check if the Kustomization has wait set to true and if so, recommend to set a timeout value.
4. Check if the Kustomization has prune set to true and if so, recommend to set spec.deletionPolicy to Delete.
5. Check if the Kustomization has force set to true and if so, recommend to remove it.
   Explain that force recreates resources and can cause downtime,
   it should be used only in emergencies when patching fails due to immutable field changes.
`
	case fluxcdv1.FluxHelmReleaseKind:
		return `
If asked for recommendations:
1. Check if the interval is less than 10 minutes and if so, recommend to increase it.
   Explain that the HelmRelease interval is for detecting drift in cluster.
   The interval set in the source (OCIRepository, HelmRepository) of the HelmRelease
   is for detecting changes in upstream Helm chart, and that one can be set to a lower value.
2. Check if the HelmRelease has releaseName set and if not, recommend to add it.
3. Check if the HelmRelease has targetNamespace set, if so check if storageNamespace is set to the same value.
   If not, recommend to set storageNamespace to the same value as targetNamespace.
4. Check if postRenderers are set, if any of the patches have a namespace set in the target, recommend to remove it.
`
	default:
		return ""
	}
}
