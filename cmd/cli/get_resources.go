// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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

  # List Flux resources by specific kind short names
  flux-operator -n flux-system get all --kind ks,hr,gitrepo,ocirepo

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
	err := getResourcesCmd.RegisterFlagCompletionFunc("kind", resourceKindCompletionFunc(true))
	if err != nil {
		rootCmd.PrintErrf("✗ failed to register kind completion function: %v\n", err)
	}

	getCmd.AddCommand(getResourcesCmd)
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

	var kinds []string
	for _, kind := range fluxKinds {
		if kind.Reconcilable {
			kinds = append(kinds, kind.Name)
		}
	}

	if len(getResourcesArgs.kinds) > 0 {
		var validatedKinds []string
		for _, k := range getResourcesArgs.kinds {
			kind, err := findFluxKind(k)
			if err != nil {
				return err
			}
			validatedKinds = append(validatedKinds, kind)
		}
		kinds = validatedKinds
	}

	for _, kind := range kinds {
		gvk, err := preferredFluxGVK(kind, kubeconfigArgs)
		if err != nil {
			if strings.Contains(err.Error(), "no matches for kind") {
				continue
			}
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
			rs := resourceStatusFromUnstructured(obj)

			if getResourcesArgs.readyStatus != "" && !strings.EqualFold(rs.Ready, getResourcesArgs.readyStatus) {
				continue
			}

			result = append(result, rs)
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
