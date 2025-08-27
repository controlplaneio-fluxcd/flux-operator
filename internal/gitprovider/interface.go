// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
)

// Interface that all Git SaaS providers must implement.
type Interface interface {
	// ListTags returns a list of tags that match the filters.
	ListTags(ctx context.Context, opts Options) ([]Result, error)

	// ListBranches returns a list of branches that match the filters.
	ListBranches(ctx context.Context, opts Options) ([]Result, error)

	// ListRequests returns a list of pull/merge requests that match the filters.
	ListRequests(ctx context.Context, opts Options) ([]Result, error)

	// ListEnvironments returns a list of environments that match the filters.
	ListEnvironments(ctx context.Context, opts Options) ([]Result, error)
}
