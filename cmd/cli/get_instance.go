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

var getInstanceCmd = &cobra.Command{
	Use:               "instance",
	Aliases:           []string{"instances"},
	Short:             "List Flux instances",
	Args:              cobra.MaximumNArgs(1),
	RunE:              geInstanceCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.FluxInstanceKind)),
}

type getInstanceFlags struct {
	allNamespaces bool
}

var getInstanceArgs getInstanceFlags

func init() {
	getInstanceCmd.Flags().BoolVarP(&getInstanceArgs.allNamespaces, "all-namespaces", "A", true,
		"List instances in all namespaces.")
	getCmd.AddCommand(getInstanceCmd)
}

func geInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("a single FluxInstance name can be specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	if !getInstanceArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}
	if len(args) == 1 {
		sel, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", args[0]))
		if err != nil {
			return fmt.Errorf("unable to parse field selector: %w", err)
		}
		lsOpts.FieldSelector = sel
	}

	var list fluxcdv1.FluxInstanceList
	err = kubeClient.List(ctx, &list, lsOpts)
	if err != nil {
		return err
	}

	rows := make([][]string, 0)
	for _, obj := range list.Items {
		objCount := 0
		if obj.Status.Inventory != nil {
			objCount = len(obj.Status.Inventory.Entries)
		}
		ready := "Unknown"
		lastReconciled := "Unknown"
		if conditions.Has(&obj, "Ready") {
			ready = string(conditions.Get(&obj, "Ready").Status)
			lastReconciled = conditions.Get(&obj, "Ready").LastTransitionTime.String()
		}

		if obj.IsDisabled() {
			ready = "Suspended"
		}

		row := []string{
			obj.Name,
			strconv.Itoa(objCount),
			ready,
			conditions.GetMessage(&obj, "Ready"),
			lastReconciled,
		}
		if getInstanceArgs.allNamespaces {
			row = append([]string{obj.Namespace}, row...)
		}
		rows = append(rows, row)
	}

	header := []string{"Name", "Resources", "Ready", "Message", "Last Reconciled"}
	if getInstanceArgs.allNamespaces {
		header = append([]string{"Namespace"}, header...)
	}

	printTable(rootCmd.OutOrStdout(), header, rows)

	return nil
}
