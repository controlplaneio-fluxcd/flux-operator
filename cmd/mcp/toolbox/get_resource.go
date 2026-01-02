// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

const (
	// ToolGetKubernetesResources is the name of the get_kubernetes_resources tool.
	ToolGetKubernetesResources = "get_kubernetes_resources"
)

func init() {
	systemTools[ToolGetKubernetesResources] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// getKubernetesResourcesInput defines the input parameters for retrieving Kubernetes resources.
type getKubernetesResourcesInput struct {
	APIVersion string  `json:"apiVersion" jsonschema:"The apiVersion of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available apiVersions."`
	Kind       string  `json:"kind" jsonschema:"The kind of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available kinds."`
	Name       string  `json:"name,omitempty" jsonschema:"The name of the Kubernetes resource."`
	Namespace  string  `json:"namespace,omitempty" jsonschema:"The namespace of the Kubernetes resource."`
	Selector   string  `json:"selector,omitempty" jsonschema:"The label selector in the format key1=value1 key2=value2."`
	Limit      float64 `json:"limit,omitempty" jsonschema:"The maximum number of resources to return."`
}

// HandleGetKubernetesResources is the handler function for the get_kubernetes_resources tool.
func (m *Manager) HandleGetKubernetesResources(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesResourcesInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesResources, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.APIVersion == "" {
		return NewToolResultError("apiVersion is required")
	}
	if input.Kind == "" {
		return NewToolResultError("kind is required")
	}
	limit := int(input.Limit)

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	gvk, err := kubeClient.ParseGroupVersionKind(input.APIVersion, input.Kind)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to parse group version kind", err)
	}

	result, err := kubeClient.Export(ctx,
		[]schema.GroupVersionKind{gvk},
		input.Name,
		input.Namespace,
		input.Selector,
		limit,
		m.maskSecrets)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to export resources", err)
	}

	if result == "" {
		return NewToolResultError("No resources found")
	}

	return NewToolResultText(result + m.GetFluxTip(input.Kind))
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
