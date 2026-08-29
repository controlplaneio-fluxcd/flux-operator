// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolPatchKubernetesResource is the name of the patch_kubernetes_resource tool.
	ToolPatchKubernetesResource = "patch_kubernetes_resource"
)

func init() {
	systemTools[ToolPatchKubernetesResource] = systemTool{
		readOnly:  false,
		inCluster: true,
	}
}

// patchKubernetesResourceInput defines the input parameters for patching a Kubernetes resource.
type patchKubernetesResourceInput struct {
	APIVersion  string `json:"apiVersion" jsonschema:"The apiVersion of the resource to patch."`
	Kind        string `json:"kind" jsonschema:"The kind of the resource to patch."`
	Name        string `json:"name" jsonschema:"The name of the resource to patch."`
	Namespace   string `json:"namespace,omitempty" jsonschema:"The namespace of the resource; omit for cluster-scoped resources."`
	Patch       string `json:"patch" jsonschema:"The patch body as a YAML or JSON string."`
	Type        string `json:"type,omitempty" jsonschema:"Patch type: merge (RFC 7386, default), json (RFC 6902 operation list) or strategic (built-in kinds only)."`
	Subresource string `json:"subresource,omitempty" jsonschema:"Subresource to patch; only status is supported."`
	DryRun      bool   `json:"dry_run,omitempty" jsonschema:"Preview the patch with server-side dry run without persisting it."`
	Overwrite   bool   `json:"overwrite,omitempty" jsonschema:"Allow patching resources managed by Flux."`
}

// HandlePatchKubernetesResource is the handler function for the patch_kubernetes_resource tool.
func (m *Manager) HandlePatchKubernetesResource(ctx context.Context, request *mcp.CallToolRequest, input patchKubernetesResourceInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolPatchKubernetesResource, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.APIVersion == "" {
		return NewToolResultError("apiVersion is required")
	}
	if input.Kind == "" {
		return NewToolResultError("kind is required")
	}
	if input.Name == "" {
		return NewToolResultError("name is required")
	}
	if strings.TrimSpace(input.Patch) == "" {
		return NewToolResultError("patch is required")
	}
	if input.Type != "" && input.Type != "merge" && input.Type != "json" && input.Type != "strategic" {
		return NewToolResultError("type must be merge, json or strategic")
	}
	if input.Subresource != "" && input.Subresource != "status" {
		return NewToolResultError("subresource must be status")
	}

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

	result, err := kubeClient.Patch(ctx, k8s.PatchRequest{
		GVK:         gvk,
		Name:        input.Name,
		Namespace:   input.Namespace,
		Patch:       input.Patch,
		Type:        input.Type,
		Subresource: input.Subresource,
		DryRun:      input.DryRun,
		Overwrite:   input.Overwrite,
	})
	if err != nil {
		return NewToolResultErrorFromErr("Failed to patch resource", err)
	}

	return NewToolResultText(result)
}
