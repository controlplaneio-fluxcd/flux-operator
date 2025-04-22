// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type GetKubernetesResourceArgs struct {
	ApiVersion    string `json:"apiVersion" jsonschema:"required,description=The apiVersion of the resource to get."`
	Kind          string `json:"kind" jsonschema:"required,description=The kind of the resource to get."`
	Name          string `json:"name" jsonschema:"description=The name of the resource to get."`
	Namespace     string `json:"namespace" jsonschema:"description=The namespace of the resource to get."`
	LabelSelector string `json:"labelSelector" jsonschema:"description=The label selector in the format label-name=label-value."`
}

func GetKubernetesResourceHandler(ctx context.Context, args GetKubernetesResourceArgs) (*mcpgolang.ToolResponse, error) {
	if args.ApiVersion == "" {
		return nil, fmt.Errorf("apiVersion is required")
	}
	if args.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}

	gv, err := schema.ParseGroupVersion(args.ApiVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to parse group version %s error: %w", args.ApiVersion, err)
	}

	gvk := metav1.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    args.Kind,
	}

	result, err := exportObjects(ctx, args.Name, args.Namespace, args.LabelSelector, []metav1.GroupVersionKind{gvk})
	if err != nil {
		return nil, fmt.Errorf("error exporting objects: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}
