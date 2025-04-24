// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpgolang "github.com/metoro-io/mcp-golang"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type GetApiVersionsArgs struct {
}

func GetApiVersionsHandler(ctx context.Context, args GetApiVersionsArgs) (*mcpgolang.ToolResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return nil, fmt.Errorf("unable to create kube client error: %w", err)
	}

	var list apiextensionsv1.CustomResourceDefinitionList
	if err := kubeClient.List(ctx, &list, client.InNamespace("")); err != nil {
		return nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	if len(list.Items) == 0 {
		return nil, errors.New("no CRDs found")
	}

	gvkList := make([]metav1.GroupVersionKind, len(list.Items))
	for i, crd := range list.Items {
		gvk := metav1.GroupVersionKind{
			Group: crd.Spec.Group,
			Kind:  crd.Spec.Names.Kind,
		}
		versions := crd.Status.StoredVersions
		if len(versions) > 0 {
			gvk.Version = versions[len(versions)-1]
		} else {
			return nil, fmt.Errorf("no stored versions found for CRD %s", crd.Name)
		}
		gvkList[i] = gvk
	}

	var strBuilder strings.Builder
	for _, gvk := range gvkList {
		itemBytes, err := yaml.Marshal(gvk)
		if err != nil {
			return nil, fmt.Errorf("error marshalling gvk: %w", err)
		}
		strBuilder.WriteString("---\n")
		strBuilder.Write(itemBytes)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(strBuilder.String())), nil
}
