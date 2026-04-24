// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/fluxcd/pkg/auth/aws"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ValidateSync validates the sync options against the Flux version in the
// Options struct, returning an error when the configuration is not supported.
func (o *Options) ValidateSync() error {
	if o.Sync == nil {
		return nil
	}

	if o.Sync.Kind == fluxcdv1.FluxGitRepositoryKind && o.Sync.Provider == aws.ProviderName {
		version, err := semver.NewVersion(o.Version)
		if err != nil {
			return fmt.Errorf("failed to parse Flux version '%s': %w", o.Version, err)
		}
		ok, err := checkVersionAgainstConstraint(version, ">= 2.9.0")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("sync.provider 'aws' for GitRepository requires Flux version >= 2.9.0, got '%s'", o.Version)
		}
	}

	return nil
}
