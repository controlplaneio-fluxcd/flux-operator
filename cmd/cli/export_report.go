// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var exportReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Export the FluxReport resource in YAML format",
	Example: `  # Export the FluxReport to standard output
  flux-operator export report

  # Export the FluxReport to a YAML file
  flux-operator export report > flux-report.yaml`,
	RunE: exportReportCmdRun,
	Args: cobra.NoArgs,
}

func init() {
	exportCmd.AddCommand(exportReportCmd)
}

func exportReportCmdRun(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	kubeClient, err := newKubeClient()
	if err != nil {
		return fmt.Errorf("unable to create kube client: %w", err)
	}

	var reportList unstructured.UnstructuredList
	reportList.SetGroupVersionKind(fluxcdv1.GroupVersion.WithKind("FluxReportList"))

	if err := kubeClient.List(ctx, &reportList); err != nil {
		return fmt.Errorf("unable to list FluxReport resources: %w", err)
	}

	if len(reportList.Items) == 0 {
		return fmt.Errorf("no FluxReport resources found")
	}

	report := &reportList.Items[0]
	cleanObjectForExport(report)

	output, err := yaml.Marshal(report.Object)
	if err != nil {
		return fmt.Errorf("unable to marshal output to YAML: %w", err)
	}

	_, err = rootCmd.OutOrStdout().Write(output)
	return err
}
