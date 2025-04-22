// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetFluxSourcesHandler(ctx context.Context, args GetFluxResourceArgs) (*mcpgolang.ToolResponse, error) {
	result, err := exportObjects(ctx, args.Name, args.Namespace, args.LabelSelector, []metav1.GroupVersionKind{
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "GitRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1beta2",
			Kind:    "OCIRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Bucket",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmRepository",
		},
		{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmChart",
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
1. Check if the interval is less than 1 minute and if so, recommend to increase it to one minute.
2. Check if the GitRepository has ref.branch set and if so, recommend to set ref.name to refs/heads/<branch name>.
3. Check if the GitRepository has ref.tag set and if so, recommend to set ref.name to refs/tags/<tag name>.
`,
		},
	}), nil
}
