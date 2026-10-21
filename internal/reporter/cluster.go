// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package reporter

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

func (r *FluxStatusReporter) getClusterInfo(ctx context.Context) (*fluxcdv1.ClusterInfo, error) {
	restConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	sv, err := clientSet.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	nodes, err := r.countNodes(ctx)
	if err != nil {
		return nil, err
	}

	return &fluxcdv1.ClusterInfo{
		ServerVersion: sv.String(),
		Platform:      sv.Platform,
		Nodes:         nodes,
	}, nil
}

// countNodes returns the number of nodes in the cluster. The nodes are
// listed as metadata only, so that the operator doesn't cache full Node
// objects when the web server is disabled.
func (r *FluxStatusReporter) countNodes(ctx context.Context) (int, error) {
	nodes := &metav1.PartialObjectMetadataList{}
	nodes.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "NodeList",
	})
	if err := r.List(ctx, nodes); err != nil {
		return 0, fmt.Errorf("failed to list nodes: %w", err)
	}
	return len(nodes.Items), nil
}
