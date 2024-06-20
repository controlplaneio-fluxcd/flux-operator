// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// FluxStatusReporter is responsible for computing
// the status report of the Flux installation.
type FluxStatusReporter struct {
	client.Client

	instance      string
	manager       string
	namespace     string
	labelSelector client.MatchingLabels
}

// NewFluxStatusReporter creates a new FluxStatusReporter
// for the given instance and namespace.
func NewFluxStatusReporter(kubeClient client.Client, instance, manager, namespace string) *FluxStatusReporter {
	return &FluxStatusReporter{
		Client:        kubeClient,
		instance:      instance,
		manager:       manager,
		namespace:     namespace,
		labelSelector: client.MatchingLabels{"app.kubernetes.io/part-of": instance},
	}
}

// Compute generate the status report of the Flux installation.
func (r *FluxStatusReporter) Compute(ctx context.Context) (fluxcdv1.FluxReportSpec, error) {
	report := fluxcdv1.FluxReportSpec{}
	report.Distribution = r.getDistributionStatus(ctx)

	crds, err := r.listCRDs(ctx)
	if err != nil {
		return report, fmt.Errorf("failed to list CRDs: %w", err)
	}

	componentsStatus, err := r.getComponentsStatus(ctx)
	if err != nil {
		return report, fmt.Errorf("failed to compute components status: %w", err)
	}
	report.ComponentsStatus = componentsStatus

	reconcilersStatus, err := r.getReconcilersStatus(ctx, crds)
	if err != nil {
		return report, fmt.Errorf("failed to compute reconcilers status: %w", err)
	}
	report.ReconcilersStatus = reconcilersStatus

	syncStatus, err := r.getSyncStatus(ctx, crds)
	if err != nil {
		return report, fmt.Errorf("failed to compute sync status: %w", err)
	}
	report.SyncStatus = syncStatus

	return report, nil
}
