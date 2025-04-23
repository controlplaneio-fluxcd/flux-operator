// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	mcpgolang "github.com/metoro-io/mcp-golang"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

type SuspendResumeTool struct {
	Name        string
	Description string
	Handler     any
}

var SuspendResumeToolList = []SuspendResumeTool{
	{
		Name:        "suspend-flux-reconciliation",
		Description: "This tool suspends the reconciliation of a Flux resource identified by apiVersion, kind, name and namespace.",
		Handler:     SuspendReconciliationHandler,
	},
	{
		Name:        "resume-flux-reconciliation",
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

	gvk := schema.FromAPIVersionAndKind(args.ApiVersion, args.Kind)
	err := toggleSuspension(ctx, gvk, args.Name, args.Namespace, true)
	if err != nil {
		return nil, fmt.Errorf("unable to suspend reconciliation: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("Reconciliation of %s/%s/%s suspended", gvk.Kind, args.Namespace, args.Name))), nil
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

	gvk := schema.FromAPIVersionAndKind(args.ApiVersion, args.Kind)
	err := toggleSuspension(ctx, gvk, args.Name, args.Namespace, false)
	if err != nil {
		return nil, fmt.Errorf("unable to resume reconciliation: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent(fmt.Sprintf("Reconciliation of %s/%s/%s resumed and started", gvk.Kind, args.Namespace, args.Name))), nil
}

func toggleSuspension(ctx context.Context, gvk schema.GroupVersionKind, name, namespace string, suspend bool) error {
	if strings.EqualFold(gvk.Group, fluxcdv1.GroupVersion.Group) {
		val := fluxcdv1.EnabledValue
		if suspend {
			val = fluxcdv1.DisabledValue
		}
		return annotateResource(ctx,
			gvk.Group,
			gvk.Version,
			gvk.Kind,
			name,
			namespace,
			[]string{fluxcdv1.ReconcileAnnotation},
			val)
	}

	resource := &unstructured.Unstructured{}
	resource.SetGroupVersionKind(gvk)

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	patch := client.MergeFrom(resource.DeepCopy())

	if suspend {
		err = unstructured.SetNestedField(resource.Object, suspend, "spec", "suspend")
		if err != nil {
			return fmt.Errorf("unable to set suspend field: %w", err)
		}
	} else {
		unstructured.RemoveNestedField(resource.Object, "spec", "suspend")
	}

	if err := kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to patch %s/%s/%s error: %w", gvk.Kind, namespace, name, err)
	}

	return nil
}
