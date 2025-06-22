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

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the client and server version information",
	Args:  cobra.NoArgs,
	RunE:  versionCmdRun,
}

type versionFlags struct {
	clientOnly bool
}

var versionArgs versionFlags

func init() {
	versionCmd.Flags().BoolVar(&versionArgs.clientOnly, "client", false,
		"If true, shows client version only (no server required).")
	rootCmd.AddCommand(versionCmd)
}

func versionCmdRun(cmd *cobra.Command, args []string) error {
	_, err := fmt.Fprintln(rootCmd.OutOrStdout(), "client:", VERSION)
	if err != nil {
		return fmt.Errorf("failed to print client version: %w", err)
	}

	if versionArgs.clientOnly {
		return nil
	}

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
	if report.Spec.Operator == nil || report.Spec.Operator.Version == "" {
		return fmt.Errorf("operator version not found in FluxReport resource")
	}

	_, err = fmt.Fprintln(rootCmd.OutOrStdout(), "server:", report.Spec.Operator.Version)
	if err != nil {
		return fmt.Errorf("failed to print server version: %w", err)
	}

	if report.Spec.Distribution.Version != "" {
		_, err = fmt.Fprintln(rootCmd.OutOrStdout(), "distribution:", report.Spec.Distribution.Version)
		if err != nil {
			return fmt.Errorf("failed to print distribution version: %w", err)
		}
	}

	return nil
}
