// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	mcpgolang "github.com/metoro-io/mcp-golang"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action struct {
	Name        string
	Description string
	Handler     any
}

var ActionList = []Action{
	{
		Name:        "reconcile-flux-object",
		Description: "This tool triggers the reconciliation of a Flux object identified by kind, name and namespace.",
		Handler:     ReconcileActionHandler,
	},
}

type ActionArgs struct {
	Kind      string `json:"kind" jsonschema:"required,description=The Flux object kind. Don't guess it can only be one of Kustomization, HelmRelease, GitRepository, OCIRepository, Bucket."`
	Name      string `json:"name" jsonschema:"required,description=The name of the Flux object."`
	Namespace string `json:"namespace" jsonschema:"required,description=The namespace of the Flux object."`
}

func ReconcileActionHandler(ctx context.Context, args ActionArgs) (*mcpgolang.ToolResponse, error) {
	if args.Kind == "" {
		return nil, errors.New("kind is required")
	}
	if args.Name == "" {
		return nil, errors.New("name is required")
	}
	if args.Namespace == "" {
		return nil, errors.New("namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	var err error
	switch {
	case strings.Contains(strings.ToLower(args.Kind), "kustomization"):
		err = annotateResource(ctx,
			"kustomize.toolkit.fluxcd.io",
			"v1",
			"Kustomization",
			args.Name,
			args.Namespace,
			meta.ReconcileRequestAnnotation,
			metav1.Now().String())
	case strings.Contains(strings.ToLower(args.Kind), "helmrelease"):
		err = annotateResource(ctx,
			"helm.toolkit.fluxcd.io",
			"v2",
			"HelmRelease",
			args.Name,
			args.Namespace,
			meta.ReconcileRequestAnnotation,
			metav1.Now().String())
	case strings.Contains(strings.ToLower(args.Kind), "gitrepository"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1",
			"GitRepository",
			args.Name,
			args.Namespace,
			meta.ReconcileRequestAnnotation,
			metav1.Now().String())
	case strings.Contains(strings.ToLower(args.Kind), "bucket"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1",
			"Bucket",
			args.Name,
			args.Namespace,
			meta.ReconcileRequestAnnotation,
			metav1.Now().String())
	case strings.Contains(strings.ToLower(args.Kind), "ocirepository"):
		err = annotateResource(ctx,
			"source.toolkit.fluxcd.io",
			"v1beta2",
			"OCIRepository",
			args.Name,
			args.Namespace,
			meta.ReconcileRequestAnnotation,
			metav1.Now().String())
	default:
		return nil, fmt.Errorf("unknown kind %s", args.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to annotate resource: %w", err)
	}

	return mcpgolang.NewToolResponse(mcpgolang.NewTextContent("reconciliation triggered")), nil
}

func annotateResource(ctx context.Context, group, version, kind, name, namespace, key, val string) error {
	resource := &metav1.PartialObjectMetadata{}
	resource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	})

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	if err := kubeClient.Get(ctx, objectKey, resource); err != nil {
		return fmt.Errorf("unable to read %s/%s/%s error: %w", kind, namespace, name, err)
	}

	patch := client.MergeFrom(resource.DeepCopy())

	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = val
	resource.SetAnnotations(annotations)

	if err := kubeClient.Patch(ctx, resource, patch); err != nil {
		return fmt.Errorf("unable to annotate %s/%s/%s error: %w", kind, namespace, name, err)
	}

	return nil
}
