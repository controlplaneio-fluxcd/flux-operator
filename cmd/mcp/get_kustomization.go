// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetFluxKustomizationsHandler(ctx context.Context, args GetFluxResourceArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Name, args.Namespace, args.LabelSelector, []metav1.GroupVersionKind{
		{
			Group:   "kustomize.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Kustomization",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result), &mcpgolang.Content{
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
	}), nil
}
