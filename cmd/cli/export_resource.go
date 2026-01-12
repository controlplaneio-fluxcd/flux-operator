// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var exportResourceCmd = &cobra.Command{
	Use:   "resource [kind/name]",
	Short: "Export a Flux custom resource in YAML or JSON format",
	Example: `  # Export a ResourceSet to standard output
  flux-operator -n flux-system export resource ResourceSet/apps

  # Export a ResourceSet in JSON format
  flux-operator -n flux-system export resource ResourceSet/apps -o json

  # Export a OCIRepository to a YAML file
  flux-operator -n apps export resource OCIRepository/my-app > my-app.yaml
`,
	Args:              cobra.ExactArgs(1),
	RunE:              exportResourceCmdRun,
	ValidArgsFunction: resourceKindNameCompletionFunc(false),
}

type exportResourceFlags struct {
	output string
}

var exportResourceArgs exportResourceFlags

func init() {
	exportResourceCmd.Flags().StringVarP(&exportResourceArgs.output, "output", "o", "yaml",
		"Output format. One of: yaml, json.")
	exportCmd.AddCommand(exportResourceCmd)
}

func exportResourceCmdRun(_ *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("resource name is required")
	}

	parts := strings.Split(args[0], "/")
	if len(parts) != 2 {
		return fmt.Errorf("resource name must be in the format <kind>/<name>, e.g., ResourceSet/my-app")
	}

	kind := parts[0]
	name := parts[1]

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("unable to get gvk for kind %s: %w", kind, err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(*gvk)
	objKey := client.ObjectKey{
		Namespace: *kubeconfigArgs.Namespace,
		Name:      name,
	}

	if err := kubeClient.Get(ctx, objKey, obj); err != nil {
		return fmt.Errorf("unable to get resource %s/%s: %w", kind, name, err)
	}

	cleanObjectForExport(obj)

	var output []byte
	switch exportResourceArgs.output {
	case "json":
		output, err = json.MarshalIndent(obj.Object, "", "  ")
		if err != nil {
			return fmt.Errorf("unable to marshal output to JSON: %w", err)
		}
	case "yaml":
		output, err = yaml.Marshal(obj.Object)
		if err != nil {
			return fmt.Errorf("unable to marshal output to YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported output format: %s", exportResourceArgs.output)
	}

	_, err = rootCmd.OutOrStdout().Write(output)
	return err
}

// cleanObjectForExport removes fields that shouldn't be included in exports
func cleanObjectForExport(obj *unstructured.Unstructured) {
	// Remove status subresource
	unstructured.RemoveNestedField(obj.Object, "status")

	// Remove runtime metadata - keep only name, namespace, labels, and annotations
	metadata := obj.Object["metadata"].(map[string]any)
	cleanMetadata := make(map[string]any)

	// Preserve essential fields
	if name, exists := metadata["name"]; exists {
		cleanMetadata["name"] = name
	}
	if namespace, exists := metadata["namespace"]; exists {
		cleanMetadata["namespace"] = namespace
	}
	if labels, exists := metadata["labels"]; exists {
		cleanMetadata["labels"] = labels
	}
	if annotations, exists := metadata["annotations"]; exists {
		cleanMetadata["annotations"] = annotations
	}

	// Remove Flux CLI annotations from clean metadata
	if annotations, exists := cleanMetadata["annotations"]; exists {
		if annotationMap, ok := annotations.(map[string]any); ok {
			delete(annotationMap, meta.ReconcileRequestAnnotation)
			// Remove annotations map if empty after cleanup
			if len(annotationMap) == 0 {
				delete(cleanMetadata, "annotations")
			}
		}
	}

	// Remove Flux ownership labels from clean metadata
	if labels, exists := cleanMetadata["labels"]; exists {
		if labelMap, ok := labels.(map[string]any); ok {
			for key := range labelMap {
				if fluxcdv1.IsFluxAPI(key) &&
					(strings.HasSuffix(key, "/name") || strings.HasSuffix(key, "/namespace")) {
					delete(labelMap, key)
				}
			}
			// Remove labels map if empty after cleanup
			if len(labelMap) == 0 {
				delete(cleanMetadata, "labels")
			}
		}
	}

	// Replace metadata with the clean version
	obj.Object["metadata"] = cleanMetadata
}
