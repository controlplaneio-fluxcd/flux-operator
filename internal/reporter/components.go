// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"cmp"
	"context"
	"fmt"

	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func (r *FluxStatusReporter) getComponentsStatus(ctx context.Context) ([]fluxcdv1.FluxComponentStatus, error) {
	deployments := unstructured.UnstructuredList{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
		},
	}

	if err := r.List(ctx, &deployments, client.InNamespace(r.namespace), r.labelSelector); err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	components := make([]fluxcdv1.FluxComponentStatus, len(deployments.Items))
	for i, d := range deployments.Items {
		res, err := status.Compute(&d)
		if err != nil {
			components[i] = fluxcdv1.FluxComponentStatus{
				Name:   d.GetName(),
				Ready:  false,
				Status: fmt.Sprintf("Failed to compute status: %s", err.Error()),
			}
		} else {
			components[i] = fluxcdv1.FluxComponentStatus{
				Name:   d.GetName(),
				Ready:  res.Status == status.CurrentStatus,
				Status: fmt.Sprintf("%s %s", string(res.Status), res.Message),
			}
		}

		containers, found, _ := unstructured.NestedSlice(d.Object, "spec", "template", "spec", "containers")
		if found && len(containers) > 0 {
			components[i].Image = containers[0].(map[string]any)["image"].(string)
		}
	}

	slices.SortStableFunc(components, func(i, j fluxcdv1.FluxComponentStatus) int {
		return cmp.Compare(i.Name, j.Name)
	})

	return components, nil
}
