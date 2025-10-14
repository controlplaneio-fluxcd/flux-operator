// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package install

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxcd/pkg/ssa"
	ssautil "github.com/fluxcd/pkg/ssa/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ApplyOperator applies the Flux Operator manifests to the cluster.
// It sets consistent labels on all objects and ensures Deployment resources have matching selector and template labels.
func (in *Installer) ApplyOperator(ctx context.Context, objects []*unstructured.Unstructured) (*ssa.ChangeSet, error) {
	labels := map[string]string{
		"app.kubernetes.io/name":     "flux-operator",
		"app.kubernetes.io/instance": "flux-operator",
	}
	ssautil.SetCommonMetadata(objects, labels, nil)

	// Iterate through objects and set label selectors to ensure
	// that helm-controller can adopt the Flux Operator deployment
	for _, obj := range objects {
		if obj.GetKind() == "Deployment" {
			// Set spec.selector.matchLabels
			if err := unstructured.SetNestedStringMap(obj.Object, labels, "spec", "selector", "matchLabels"); err != nil {
				return nil, fmt.Errorf("failed to set deployment selector labels: %w", err)
			}

			// Set spec.template.metadata.labels
			if err := unstructured.SetNestedStringMap(obj.Object, labels, "spec", "template", "metadata", "labels"); err != nil {
				return nil, fmt.Errorf("failed to set deployment template labels: %w", err)
			}
		}
		if obj.GetKind() == "Service" {
			// Set spec.selector
			if err := unstructured.SetNestedStringMap(obj.Object, labels, "spec", "selector"); err != nil {
				return nil, fmt.Errorf("failed to set service selector labels: %w", err)
			}
		}
	}

	return in.kubeClient.Manager.ApplyAllStaged(ctx, objects, ssa.DefaultApplyOptions())
}

// ApplyInstance applies the Flux instance manifests to the cluster.
func (in *Installer) ApplyInstance(ctx context.Context, instance *fluxcdv1.FluxInstance) (*ssa.ChangeSet, error) {
	// Convert to unstructured
	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(instance)
	if err != nil {
		return nil, err
	}

	return in.kubeClient.Manager.ApplyAllStaged(ctx, []*unstructured.Unstructured{{Object: rawMap}}, ssa.DefaultApplyOptions())
}

// WaitFor waits for all resources in the provided ChangeSet to be ready within the given timeout.
func (in *Installer) WaitFor(ctx context.Context, cs *ssa.ChangeSet, timeout time.Duration) error {
	return in.kubeClient.Manager.WaitForSetWithContext(ctx, cs.ToObjMetadataSet(), ssa.WaitOptions{
		Interval: 5 * time.Second,
		Timeout:  timeout,
	})
}
