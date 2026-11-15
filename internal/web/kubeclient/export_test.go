// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package kubeclient

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetNodesAccessReview replaces the list nodes access review for tests.
func SetNodesAccessReview(c *Client, fn func(ctx context.Context, kubeClient client.Client) (bool, error)) {
	c.nodesAccessReview = fn
}
