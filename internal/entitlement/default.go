// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package entitlement

import (
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
)

// DefaultClient is an offline entitlement client.
// This client uses a SHA256 digest to generate and verify tokens.
type DefaultClient struct {
	Vendor string
}

// RegisterUsage registers the usage with the default entitlement client.
func (c *DefaultClient) RegisterUsage(ctx context.Context, id string) (string, error) {
	d := digest.FromString(fmt.Sprintf("%s-%s", c.Vendor, id))
	return d.Encoded(), nil
}

// Verify verifies the token matches the SHA256 digest of the vendor id.
func (c *DefaultClient) Verify(token, id string) (bool, error) {
	d := digest.FromString(fmt.Sprintf("%s-%s", c.Vendor, id))
	return token == d.Encoded(), nil
}

// GetVendor returns the vendor name.
func (c *DefaultClient) GetVendor() string {
	return c.Vendor
}
