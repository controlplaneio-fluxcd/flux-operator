// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Print Flux custom resources statistics",
	Args:  cobra.NoArgs,
	RunE:  statsCmdRun,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func statsCmdRun(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client error: %w", err)
	}

	lsOpts := &client.ListOptions{}
	var list fluxcdv1.FluxReportList
	err = kubeClient.List(ctx, &list, lsOpts)
	if err != nil {
		return fmt.Errorf("failed to get FluxReport resource: %w", err)
	}

	if len(list.Items) == 0 {
		return fmt.Errorf("no FluxReport resources found, ensure the Flux Operator is installed")
	}

	report := list.Items[0]
	if len(report.Spec.ReconcilersStatus) == 0 {
		return fmt.Errorf("no stats found in the FluxReport")
	}

	rows := make([][]string, 0)
	for _, status := range report.Spec.ReconcilersStatus {
		storage := "-"
		if status.Stats.TotalSize != "" {
			storage = status.Stats.TotalSize
		}
		row := []string{
			status.Kind,
			fmt.Sprintf("%d", status.Stats.Running),
			fmt.Sprintf("%d", status.Stats.Failing),
			fmt.Sprintf("%d", status.Stats.Suspended),
			storage,
		}
		rows = append(rows, row)
	}

	header := []string{"Reconcilers", "Running", "Failing", "Suspended", "Storage"}
	printTable(rootCmd.OutOrStdout(), header, rows)

	return nil
}
