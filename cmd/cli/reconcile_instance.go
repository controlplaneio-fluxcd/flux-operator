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

var reconcileInstanceCmd = &cobra.Command{
	Use:   "instance",
	Short: "Trigger FluxInstance reconciliation",
	Example: `  # Trigger the reconciliation of an instance
  flux-operator -n flux-system reconcile instance flux
`,
	RunE: reconcileInstanceCmdRun,
}

func init() {
	reconcileCmd.AddCommand(reconcileInstanceCmd)
}

func reconcileInstanceCmdRun(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("name is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	return annotateResource(ctx,
		fluxcdv1.FluxInstanceKind, args[0],
		*kubeconfigArgs.Namespace,
		meta.ReconcileRequestAnnotation,
		metav1.Now().String())
}
