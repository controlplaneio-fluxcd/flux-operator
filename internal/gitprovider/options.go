// Copyright 2024 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package gitprovider

import (
	"crypto/x509"

	"github.com/controlplaneio-fluxcd/flux-operator/internal/filtering"
)

// Options holds the configuration for the Git SaaS provider.
type Options struct {
	URL      string
	CertPool *x509.CertPool
	Token    string
	Filters  filtering.Filters
}
