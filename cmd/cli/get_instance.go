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
	for _, instance := range list.Items {
		objCount := 0
		if instance.Status.Inventory != nil {
			objCount = len(instance.Status.Inventory.Entries)
		}
		ready := "Unknown"
		if conditions.Has(&instance, "Ready") {
			ready = string(conditions.Get(&instance, "Ready").Status)
		}
		row := []string{
			instance.Name,
			strconv.Itoa(objCount),
			ready,
			conditions.GetMessage(&instance, "Ready"),
		}
		if getInstanceArgs.allNamespaces {
			row = append([]string{instance.Namespace}, row...)
		}
		rows = append(rows, row)
	}

	header := []string{"Name", "Resources", "Ready", "Message"}
	if getInstanceArgs.allNamespaces {
		header = append([]string{"Namespace"}, header...)
	}

	printTable(rootCmd.OutOrStdout(), header, rows)

	return nil
}
