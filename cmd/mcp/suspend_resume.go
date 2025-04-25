// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	mcpgolang "github.com/metoro-io/mcp-golang"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/client"
)

type SuspendResumeTool struct {
	Name        string
	Description string
	Handler     any
}

var SuspendResumeToolList = []SuspendResumeTool{
	{
		Name:        "suspend_flux_reconciliation",
		Description: "This tool suspends the reconciliation of a Flux resource identified by apiVersion, kind, name and namespace.",
		Handler:     SuspendReconciliationHandler,
	},
	{
		Name:        "resume_flux_reconciliation",
		Description: "This tool resumes the reconciliation of a Flux resource identified by apiVersion, kind, name and namespace.",
		Handler:     ResumeReconciliationHandler,
	},
}

type SuspendResumeReconciliationArgs struct {
	ApiVersion string `json:"apiVersion" jsonschema:"required,description=The apiVersion of the Flux resource."`
	Kind       string `json:"kind" jsonschema:"required,description=The kind of the Flux resource."`
	Name       string `json:"name" jsonschema:"required,description=The name of the Flux resource."`
	Namespace  string `json:"namespace" jsonschema:"required,description=The namespace of the Flux resource."`
}

func SuspendReconciliationHandler(ctx context.Context, args SuspendResumeReconciliationArgs) (*mcpgolang.ToolResponse, error) {
	if args.ApiVersion == "" {
		return nil, fmt.Errorf("apiVersion is required")
	}
	if args.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}
	if args.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if args.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	gvk, err := kubeClient.ParseGroupVersionKind(args.ApiVersion, args.Kind)
	if err != nil {
		return nil, fmt.Errorf("unable to parse group version kind %s/%s: %w", args.ApiVersion, args.Kind, err)
	}

	err = kubeClient.ToggleSuspension(ctx, gvk, args.Name, args.Namespace, true)
	if err != nil {
		return nil, fmt.Errorf("unable to suspend reconciliation: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf(
		"Reconciliation of %s/%s/%s suspended.To resume reconciliation, run the resume_flux_reconciliation tool.",
		gvk.Kind, args.Namespace, args.Name))), nil
}

func ResumeReconciliationHandler(ctx context.Context, args SuspendResumeReconciliationArgs) (*mcpgolang.ToolResponse, error) {
	if args.ApiVersion == "" {
		return nil, fmt.Errorf("apiVersion is required")
	}
	if args.Kind == "" {
		return nil, fmt.Errorf("kind is required")
	}
	if args.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if args.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := client.NewClient(kubeconfigArgs)
	if err != nil {
		return nil, err
	}

	gvk, err := kubeClient.ParseGroupVersionKind(args.ApiVersion, args.Kind)
	if err != nil {
		return nil, fmt.Errorf("unable to parse group version kind %s/%s: %w", args.ApiVersion, args.Kind, err)
	}

	err = kubeClient.ToggleSuspension(ctx, gvk, args.Name, args.Namespace, false)
	if err != nil {
		return nil, fmt.Errorf("unable to resume reconciliation: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("Reconciliation of %s/%s/%s resumed and started", gvk.Kind, args.Namespace, args.Name))), nil
}
