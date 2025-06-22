// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var resumeInputProviderCmd = &cobra.Command{
	Use:               "inputprovider",
	Aliases:           []string{"rsip", "resourcesetinputprovider"},
	Short:             "Resume ResourceSetInputProvider reconciliation",
	Args:              cobra.ExactArgs(1),
	RunE:              resumeInputProviderCmdRun,
	ValidArgsFunction: resourceNamesCompletionFunc(fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)),
}

type resumeInputProviderFlags struct {
	wait bool
}

var resumeInputProviderArgs resumeInputProviderFlags

func init() {
	resumeInputProviderCmd.Flags().BoolVar(&resumeInputProviderArgs.wait, "wait", true,
		"Wait for the resource to become ready.")
	resumeCmd.AddCommand(resumeInputProviderCmd)
}

func resumeInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := args[0]
	now := metav1.Now().String()
	gvk := fluxcdv1.GroupVersion.WithKind(fluxcdv1.ResourceSetInputProviderKind)

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	err := toggleSuspension(ctx, gvk, name, *kubeconfigArgs.Namespace, now, false)
	if err != nil {
		return err
	}

	if resumeInstanceArgs.wait {
		rootCmd.Println(`◎`, "Waiting for reconciliation...")
		msg, err := waitForResourceReconciliation(ctx,
			gvk,
			name,
			*kubeconfigArgs.Namespace,
			now,
			rootArgs.timeout)
		if err != nil {
			return err
		}

		rootCmd.Println(`✔`, msg)
	} else {
		rootCmd.Println(`✔`, "Reconciliation resumed")
	}

	return nil
}
