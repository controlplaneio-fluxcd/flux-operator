// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"context"
	"fmt"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

var reconcileInputProviderCmd = &cobra.Command{
	Use:     "inputprovider",
	Aliases: []string{"rsip", "resourcesetinputprovider"},
	Short:   "Trigger ResourceSetInputProvider reconciliation",
	RunE:    reconcileInputProviderCmdRun,
}

var reconcileInputProviderArgs struct {
	force bool
}

func init() {
	reconcileInputProviderCmd.Flags().BoolVar(&reconcileInputProviderArgs.force, "force", false,
		"Force the reconciliation of the ResourceSetInputProvider, even if the current time is outside the schedule.")

	reconcileCmd.AddCommand(reconcileInputProviderCmd)
}

func reconcileInputProviderCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	now := metav1.Now().String()

	if !reconcileInputProviderArgs.force {
		return annotateResource(ctx,
			fluxcdv1.ResourceSetInputProviderKind, args[0],
			*kubeconfigArgs.Namespace,
			meta.ReconcileRequestAnnotation,
			now)
	}

	return annotateResourceWithMap(ctx,
		fluxcdv1.ResourceSetInputProviderKind, args[0],
		*kubeconfigArgs.Namespace,
		map[string]string{
			meta.ReconcileRequestAnnotation: now,
			meta.ForceRequestAnnotation:     now,
		})
}
