// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fluxcd/pkg/runtime/conditions"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var getResourceSetCmd = &cobra.Command{
	Use:               "resourceset",
	Aliases:           []string{"resourcesets", "rset"},
	Short:             "List ResourceSets",
	RunE:              getResourceSetCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetKind)),
}

type getResourceSetFlags struct {
	allNamespaces bool
}

var getResourceSetArgs getResourceSetFlags

func init() {
	getResourceSetCmd.Flags().BoolVarP(&getResourceSetArgs.allNamespaces, "all-namespaces", "A", false,
		"List ResourceSets in all namespaces.")
	getCmd.AddCommand(getResourceSetCmd)
}

func getResourceSetCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("a single ResourceSet name can be specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	if !getResourceSetArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}
	if len(args) == 1 {
		sel, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", args[0]))
		if err != nil {
			return fmt.Errorf("unable to parse field selector: %w", err)
		}
		lsOpts.FieldSelector = sel
	}

	var list fluxcdv1.ResourceSetList
	err = kubeClient.List(ctx, &list, lsOpts)
	if err != nil {
		return err
	}

	rows := make([][]string, 0)
	for _, rset := range list.Items {
		objCount := 0
		if rset.Status.Inventory != nil {
			objCount = len(rset.Status.Inventory.Entries)
		}
		ready := "Unknown"
		lastReconciled := "Unknown"
		if conditions.Has(&rset, "Ready") {
			ready = string(conditions.Get(&rset, "Ready").Status)
			lastReconciled = conditions.Get(&rset, "Ready").LastTransitionTime.String()
		}
		row := []string{
			rset.Name,
			strconv.Itoa(objCount),
			ready,
			conditions.GetMessage(&rset, "Ready"),
			lastReconciled,
		}
		if getResourceSetArgs.allNamespaces {
			row = append([]string{rset.Namespace}, row...)
		}
		rows = append(rows, row)
	}

	header := []string{"Name", "Resources", "Ready", "Message", "Last Reconciled"}
	if getResourceSetArgs.allNamespaces {
		header = append([]string{"Namespace"}, header...)
	}

	printTable(rootCmd.OutOrStdout(), header, rows)

	return nil
}
