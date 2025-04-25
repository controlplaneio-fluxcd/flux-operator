// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type GetKubernetesResourcesArgs struct {
	ApiVersion    string `json:"apiVersion" jsonschema:"required,description=The apiVersion of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available apiVersions."`
	Kind          string `json:"kind" jsonschema:"required,description=The kind of the Kubernetes resource. Use the get_kubernetes_api_versions tool to get the available kinds."`
	Name          string `json:"name" jsonschema:"description=The name of the Kubernetes resource."`
	Namespace     string `json:"namespace" jsonschema:"description=The namespace of the Kubernetes resource."`
	LabelSelector string `json:"selector" jsonschema:"description=The label selector in the format key1=value1,key2=value2."`
}

func GetKubernetesResourcesHandler(ctx context.Context, args GetKubernetesResourcesArgs) (*mcpgolang.ToolResponse, error) {
	if args.ApiVersion == "" {
		return nil, fmt.Errorf("apiVersion is required")
	}
	if args.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}

	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	gvk, err := kubeClient.ParseGroupVersionKind(args.ApiVersion, args.Kind)
	result, err := kubeClient.Export(ctx,
		[]schema.GroupVersionKind{gvk},
		args.Name,
		args.Namespace,
		args.LabelSelector,
		rootArgs.maskSecrets)

	contents := []*mcpgolang.Content{
		mcpgolang.NewTextContent(result),
	}

	switch args.Kind {
	case "GitRepository", "Bucket", "OCIRepository":
		contents = append(contents, sourceTip)
	case "Kustomization":
		contents = append(contents, kustomizationTip)
	case "HelmRelease":
		contents = append(contents, helmReleaseTip)
	}

	return mcpgolang.NewToolResponse(contents...), nil
}

var sourceTip = &mcpgolang.Content{
	Type: mcpgolang.ContentTypeText,
	Annotations: &mcpgolang.Annotations{
		Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
	},
	TextContent: &mcpgolang.TextContent{
		Text: `If asked make the following recommendations:
1. Check if the interval is less than 1 minute and if so, recommend to increase it to one minute.
2. Check if the GitRepository has ref.branch set and if so, recommend to set ref.name to refs/heads/<branch name>.
3. Check if the GitRepository has ref.tag set and if so, recommend to set ref.name to refs/tags/<tag name>.
`,
	},
}

var kustomizationTip = &mcpgolang.Content{
	Type: mcpgolang.ContentTypeText,
	Annotations: &mcpgolang.Annotations{
		Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
	},
	TextContent: &mcpgolang.TextContent{
		Text: `If asked make the following recommendations:
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
`,
	},
}

var helmReleaseTip = &mcpgolang.Content{
	Type: mcpgolang.ContentTypeText,
	Annotations: &mcpgolang.Annotations{
		Audience: []mcpgolang.Role{mcpgolang.RoleAssistant},
	},
	TextContent: &mcpgolang.TextContent{
		Text: `If asked about container images, exact the image references as they appear in the
HelmRelease status.inventory.containers fields, with all tags preserved as they are, do not remove the ':'' or 'v'' characters, use code blocks to display them.
If asked make the following recommendations:
1. Check if the interval is less than 10 minutes and if so, recommend to increase it.
   Explain that the HelmRelease interval is for detecting drift in cluster.
   The interval set in the source (OCIRepository, HelmRepository) of the HelmRelease
   is for detecting changes in upstream Helm chart, and that one can be set to a lower value.
2. Check if the HelmRelease has releaseName set and if not, recommend to add it.
3. Check if the HelmRelease has targetNamespace set, if so check if storageNamespace is set to the same value.
   If not, recommend to set storageNamespace to the same value as targetNamespace.
4. Check if postRenderers are set, if any of the patches have a namespace set in the target, recommend to remove it.
`,
	},
}
