// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var getResourcesCmd = &cobra.Command{
	Use:     "all",
	Aliases: []string{"resources"},
	Short:   "List Flux custom resources and their status",
	Example: `  # List Flux resources in all namespaces
  flux-operator get all -A

  # List Flux resources filtered by their ready status
  flux-operator get all -A --ready-status Suspended

  # List Flux resources by specific kinds and namespace
  flux-operator -n flux-system get all --kind ResourceSet,GitRepository,Kustomization

  # List Flux resources in JSON format
  flux-operator get all -A --output json | jq
`,
	RunE: geResourcesCmdRun,
	Args: cobra.NoArgs,
}

type getResourcesFlags struct {
	allNamespaces bool
	kinds         []string
	output        string
	readyStatus   string
}

var getResourcesArgs getResourcesFlags

func init() {
	getResourcesCmd.Flags().BoolVarP(&getResourcesArgs.allNamespaces, "all-namespaces", "A", false,
		"List resources in all namespaces.")
	getResourcesCmd.Flags().StringSliceVar(&getResourcesArgs.kinds, "kind", nil,
		"List only resources of the specified kinds, accepts comma-separated values.")
	getResourcesCmd.Flags().StringVar(&getResourcesArgs.readyStatus, "ready-status", "",
		"Filter resources by their ready status, one of: True, False, Unknown, Suspended.")
	getResourcesCmd.Flags().StringVarP(&getResourcesArgs.output, "output", "o", "table",
		"Output format. One of: table, json, yaml.")
	getCmd.AddCommand(getResourcesCmd)
}

type ResourceStatus struct {
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	LastReconciled string `json:"lastReconciled"`
	Ready          string `json:"ready"`
	ReadyMessage   string `json:"message"`
}

func geResourcesCmdRun(cmd *cobra.Command, args []string) error {
	result := make([]ResourceStatus, 0)
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	if !getResourcesArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}

	kinds := []string{
		"ResourceSet",
		"ResourceSetInputProvider",
		"GitRepository",
		"OCIRepository",
		"Bucket",
		"HelmRepository",
		"HelmChart",
		"HelmRelease",
		"Kustomization",
	}
	if len(getResourcesArgs.kinds) > 0 {
		kinds = getResourcesArgs.kinds
	}

	for _, kind := range kinds {
		gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
		if err != nil {
			return fmt.Errorf("unable to get gvk for kind %s : %w", kind, err)
		}

		list := unstructured.UnstructuredList{
			Object: map[string]any{
				"apiVersion": gvk.Group + "/" + gvk.Version,
				"kind":       gvk.Kind,
			},
		}

		if err := kubeClient.List(ctx, &list, lsOpts); err != nil {
			return fmt.Errorf("unable to list resources for kind %s : %w", kind, err)
		}

		for _, obj := range list.Items {
			name := fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
			ready := "Unknown"
			readyMsg := "Not initialized"
			lastReconciled := "Unknown"
			if conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && err == nil {
				for _, cond := range conditions {
					if condition, ok := cond.(map[string]any); ok && condition["type"] == meta.ReadyCondition {
						ready = condition["status"].(string)
						if msg, exists := condition["message"]; exists {
							readyMsg = msg.(string)
						}
						if lastTransitionTime, exists := condition["lastTransitionTime"]; exists {
							lastReconciled = lastTransitionTime.(string)
						}
					}
				}
			}

			if ssautil.AnyInMetadata(&obj,
				map[string]string{fluxcdv1.ReconcileAnnotation: fluxcdv1.DisabledValue}) {
				ready = "Suspended"
			}

			if suspend, found, err := unstructured.NestedBool(obj.Object, "spec", "suspend"); suspend && found && err == nil {
				ready = "Suspended"
			}

			if getResourcesArgs.readyStatus != "" && !strings.EqualFold(ready, getResourcesArgs.readyStatus) {
				continue
			}

			result = append(result, ResourceStatus{
				Kind:           gvk.Kind,
				Name:           name,
				LastReconciled: lastReconciled,
				Ready:          ready,
				ReadyMessage:   readyMsg,
			})
		}
	}

	if len(result) == 0 {
		return fmt.Errorf("no resources found")
	}

	switch getResourcesArgs.output {
	case "json":
		output, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("unable to marshal output to JSON: %w", err)
		}
		_, err = rootCmd.OutOrStdout().Write(output)
		return err
	case "yaml":
		output, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("unable to marshal output to YAML: %w", err)
		}
		_, err = rootCmd.OutOrStdout().Write(output)
		return err
	default:
		rows := make([][]string, 0, len(result))
		for _, res := range result {
			row := []string{
				res.Kind,
				res.Name,
				res.LastReconciled,
				res.Ready,
				res.ReadyMessage,
			}
			rows = append(rows, row)
		}
		header := []string{"Kind", "Name", "Last Reconciled", "Ready", "Message"}
		printTable(rootCmd.OutOrStdout(), header, rows)
	}

	return nil
}
