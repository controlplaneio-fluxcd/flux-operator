// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"

	mcpgolang "github.com/metoro-io/mcp-golang"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type GetApiVersionsArgs struct {
}

func GetApiVersionsHandler(ctx context.Context, args GetApiVersionsArgs) (*mcpgolang.ToolResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	result, err := kubeClient.ExportAPIs(ctx)
	if err != nil {
		return nil, err
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(result)), nil
}
