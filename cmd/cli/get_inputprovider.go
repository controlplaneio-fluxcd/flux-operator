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

var getInputProviderCmd = &cobra.Command{
	Use:               "inputprovider",
	Aliases:           []string{"rsip", "resourcesetinputproviders"},
	Short:             "List ResourceSetInputProviders",
	RunE:              getInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

type getInputProviderFlags struct {
	allNamespaces bool
}

var getInputProviderArgs getInputProviderFlags

func init() {
	getInputProviderCmd.Flags().BoolVarP(&getInputProviderArgs.allNamespaces, "all-namespaces", "A", false,
		"List ResourceSetInputProviders in all namespaces.")
	getCmd.AddCommand(getInputProviderCmd)
}

func getInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("a single ResourceSetInputProvider name can be specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	if !getInputProviderArgs.allNamespaces {
		lsOpts.Namespace = *kubeconfigArgs.Namespace
	}
	if len(args) == 1 {
		sel, err := fields.ParseSelector(fmt.Sprintf("metadata.name=%s", args[0]))
		if err != nil {
			return fmt.Errorf("unable to parse field selector: %w", err)
		}
		lsOpts.FieldSelector = sel
	}

	var list fluxcdv1.ResourceSetInputProviderList
	err = kubeClient.List(ctx, &list, lsOpts)
	if err != nil {
		return err
	}

	rows := make([][]string, 0)
	for _, obj := range list.Items {
		inputsCount := 0
		if obj.Status.ExportedInputs != nil {
			inputsCount = len(obj.Status.ExportedInputs)
		}
		ready := "Unknown"
		if conditions.Has(&obj, "Ready") {
			ready = string(conditions.Get(&obj, "Ready").Status)
		}
		var nextSchedule string
		if obj.Status.NextSchedule != nil {
			nextSchedule = obj.Status.NextSchedule.When.String()
		}
		row := []string{
			obj.Name,
			strconv.Itoa(inputsCount),
			ready,
			conditions.GetMessage(&obj, "Ready"),
			nextSchedule,
		}
		if getInputProviderArgs.allNamespaces {
			row = append([]string{obj.Namespace}, row...)
		}
		rows = append(rows, row)
	}

	header := []string{"Name", "Inputs", "Ready", "Message", "Next Schedule"}
	if getInputProviderArgs.allNamespaces {
		header = append([]string{"Namespace"}, header...)
	}

	printTable(rootCmd.OutOrStdout(), header, rows)

	return nil
}
