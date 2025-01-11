// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
)

// Provider is the interface that Git SaaS providers must implement.
type Provider interface {
	// ListBranches returns a list of branches that match the filters.
	ListBranches(ctx context.Context, opts Options) ([]Result, error)

	// ListRequests returns a list of pull/merge requests that match the filters.
	ListRequests(ctx context.Context, opts Options) ([]Result, error)
}
