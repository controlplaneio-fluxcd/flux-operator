// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
func (r *FluxStatusReporter) Compute(ctx context.Context) (fluxcdv1.FluxReportSpec, []ReconcilerStatsByNamespace, error) {
	report := fluxcdv1.FluxReportSpec{}
	report.Distribution = r.getDistributionStatus(ctx)

	cluster, err := r.getClusterInfo(ctx)
	if err != nil {
		return report, nil, fmt.Errorf("failed to compute cluster info: %w", err)
	}
	report.Cluster = cluster

	crds, err := r.listCRDs(ctx)
	if err != nil {
		return report, nil, fmt.Errorf("failed to list CRDs: %w", err)
	}

	componentsStatus, err := r.getComponentsStatus(ctx)
	if err != nil {
		return report, nil, fmt.Errorf("failed to compute components status: %w", err)
	}
	report.ComponentsStatus = componentsStatus

	reconcilersStatus, reconcilersStatsByNamespace, err := r.getReconcilersStatus(ctx, crds)
	if err != nil {
		return report, nil, fmt.Errorf("failed to compute reconcilers status: %w", err)
	}
	report.ReconcilersStatus = reconcilersStatus

	syncStatus, err := r.getSyncStatus(ctx, crds)
	if err != nil {
		return report, nil, fmt.Errorf("failed to compute sync status: %w", err)
	}
	report.SyncStatus = syncStatus

	return report, reconcilersStatsByNamespace, nil
}

// RequestReportUpdate annotates the FluxReport object to trigger a reconciliation.
func RequestReportUpdate(ctx context.Context, kubeClient client.Client, instance, manager, namespace string) error {
	report := &metav1.PartialObjectMetadata{}
	report.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   fluxcdv1.GroupVersion.Group,
		Kind:    fluxcdv1.FluxReportKind,
		Version: fluxcdv1.GroupVersion.Version,
	})

	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      instance,
	}

	if err := kubeClient.Get(ctx, objectKey, report); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s '%s' error: %w", report.Kind, instance, err)
	}

	patch := client.MergeFrom(report.DeepCopy())
	annotations := report.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[meta.ReconcileRequestAnnotation] = strconv.FormatInt(metav1.Now().Unix(), 10)
	report.SetAnnotations(annotations)

	if err := kubeClient.Patch(ctx, report, patch, client.FieldOwner(manager)); err != nil {
		return fmt.Errorf("failed to annotate %s '%s' error: %w", report.Kind, instance, err)
	}
	return nil
}
