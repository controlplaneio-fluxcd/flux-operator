// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package entitlement

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	// VendorKey is the key in the entitlement secret
	// that holds the vendor name.
	VendorKey = "vendor"

	// TokenKey is the key in the entitlement secret
	// that holds the token.
	TokenKey = "token"

	// DefaultVendor is the default vendor name.
	DefaultVendor = "controlplane"

	// MarketplaceTypeEnvKey is the environment variable key
	// that holds the marketplace type.
	MarketplaceTypeEnvKey = "MARKETPLACE_TYPE"
)

// Client is the interface for entitlement clients
// that can register usage and verify tokens.
type Client interface {
	// RegisterUsage registers the usage with the entitlement service
	// and returns a signed JWT token.
	RegisterUsage(ctx context.Context, id string) (string, error)

	// Verify verifies that the token is signed by the
	// entitlement service and matches the usage id.
	Verify(token, id string) (bool, error)

	// GetVendor returns the vendor name.
	GetVendor() string
}

// NewClient returns a new entitlement client based on the
// marketplace type environment variable.
func NewClient() (Client, error) {
	vendor := DefaultVendor
	marketplace, found := os.LookupEnv(MarketplaceTypeEnvKey)
	if found && marketplace != "" && marketplace != DefaultVendor {
		vendor = fmt.Sprintf("%s-%s", DefaultVendor, strings.ToLower(marketplace))
	}

	switch vendor {
	case DefaultVendor:
		return &DefaultClient{Vendor: vendor}, nil
	case "controlplane-aws":
		return NewAmazonClient(vendor)
	}

	return nil, fmt.Errorf("unsupported vendor %s", vendor)
}
