// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GetFluxSourceArgs struct {
	Kind          string `json:"kind" jsonschema:"description=Filter by a specific Flux source kind."`
	Name          string `json:"name" jsonschema:"description=Filter by a specific name."`
	Namespace     string `json:"namespace" jsonschema:"description=Filter by a specific namespace, if not specified all namespaces are included."`
	LabelSelector string `json:"labelSelector" jsonschema:"description=The label selector in the format label-name=label-value."`
}

func GetFluxSourcesHandler(ctx context.Context, args GetFluxSourceArgs) (*mcpgolang.ToolResponse, error) {
	var sources []metav1.GroupVersionKind
	if args.Kind == "" || args.Kind == "GitRepository" {
		sources = append(sources, metav1.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "GitRepository",
		})
	}
	if args.Kind == "" || args.Kind == "Bucket" {
		sources = append(sources, metav1.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "Bucket",
		})
	}
	if args.Kind == "" || args.Kind == "HelmRepository" {
		sources = append(sources, metav1.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmRepository",
		})
	}
	if args.Kind == "" || args.Kind == "HelmChart" {
		sources = append(sources, metav1.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1",
			Kind:    "HelmChart",
		})
	}
	if args.Kind == "" || args.Kind == "OCIRepository" {
		sources = append(sources, metav1.GroupVersionKind{
			Group:   "source.toolkit.fluxcd.io",
			Version: "v1beta2",
			Kind:    "OCIRepository",
		})
	}

	result, err := exportObjects(ctx, args.Name, args.Namespace, args.LabelSelector, sources)
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
