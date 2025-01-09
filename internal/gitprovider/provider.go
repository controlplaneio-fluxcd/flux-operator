// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"context"
)

// Provider is the interface that Git SaaS providers must implement.
type Provider interface {
	ListRequests(ctx context.Context, opts Options) ([]Result, error)
}
