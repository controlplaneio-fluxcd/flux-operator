// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ValidateAndApplyWorkloadIdentityConfig validates and applies the workload identity options
// based on the provided cluster configuration and Flux version in the Options struct.
func (o *Options) ValidateAndApplyWorkloadIdentityConfig(cluster fluxcdv1.Cluster) error {
	var (
		firstControllersWithObjectLevelWorkloadIdentity = []string{
			"source-controller",
			"kustomize-controller",
			"notification-controller",
			"image-reflector-controller",
			"image-automation-controller",
		}

		allControllers = append(firstControllersWithObjectLevelWorkloadIdentity, "helm-controller")
	)

	// Parse Flux version.
	version, err := semver.NewVersion(o.Version)
	if err != nil {
		return fmt.Errorf("failed to parse Flux version '%s': %w", o.Version, err)
	}

	// Check Flux version against the necessary constraints.
	lt260, err := checkVersionAgainstConstraint(version, "< 2.6.0")
	if err != nil {
		return err
	}
	lt270, err := checkVersionAgainstConstraint(version, "< 2.7.0")
	if err != nil {
		return err
	}

	switch {

	// Handle less than 2.6.0 versions.
	case lt260:

		// Perform checks specific to this version range.
		if cluster.ObjectLevelWorkloadIdentity || cluster.MultitenantWorkloadIdentity {
			return fmt.Errorf(".objectLevelWorkloadIdentity and .multitenantWorkloadIdentity are not supported in Flux versions < 2.6.0")
		}

		// Nothing to add, nothing to remove.
		return nil

	// Handle 2.6.x versions.
	case lt270:

		// Perform checks specific to this version range.
		if cluster.MultitenantWorkloadIdentity {
			return fmt.Errorf(".multitenantWorkloadIdentity is not supported in Flux versions 2.6.x")
		}

		// In 2.6.x, if object level workload identity is not enabled, we can
		// just remove the permission for creating service account tokens, as
		// the feature gate is opt-in in this version range and hence we don't
		// need to set it to false in this case.
		if !cluster.ObjectLevelWorkloadIdentity {
			o.RemovePermissionForCreatingServiceAccountTokens = true
			return nil
		}

		// If enabled, append the patch for enabling the
		// ObjectLevelWorkloadIdentity feature gate.
		const enabled = true
		o.Patches += GetProfileObjectLevelWorkloadIdentity(firstControllersWithObjectLevelWorkloadIdentity, enabled)
		return nil

	// Handle >= 2.7.0 versions.
	default:

		// Perform checks specific to this version range.
		if !cluster.ObjectLevelWorkloadIdentity && cluster.MultitenantWorkloadIdentity {
			return fmt.Errorf(".objectLevelWorkloadIdentity must be set to true when .multitenantWorkloadIdentity is set to true")
		}

		// The feature gate may or may not be opt-in for a version in this range,
		// so we need to set it according to the cluster configuration in order
		// not to conflict with the default behavior.
		o.Patches += GetProfileObjectLevelWorkloadIdentity(allControllers, cluster.ObjectLevelWorkloadIdentity)

		// Handle object-level disabled.
		if !cluster.ObjectLevelWorkloadIdentity {
			o.RemovePermissionForCreatingServiceAccountTokens = true
			return nil
		}

		// Nothing else to do if multitenant workload identity is disabled.
		if !cluster.MultitenantWorkloadIdentity {
			return nil
		}

		// Multi-tenancy is enabled. Append default service account controller flags.
		o.Patches += GetProfileMultitenantWorkloadIdentity(cluster)
		return nil
	}
}
