// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// objectLevelWorkloadIdentityLegacyWarning is logged when the legacy
// ObjectLevelWorkloadIdentity field is used with Flux v2.10.0 or later.
const objectLevelWorkloadIdentityLegacyWarning = "warning: in Flux >=2.10, " +
	"FluxInstance.spec.cluster.objectLevelWorkloadIdentity is now considered " +
	"legacy and is redundant with the new FluxInstance.spec.cluster.objectIdentityRBAC. " +
	"their behavior is identical, so you can just change to the new field. " +
	"this is to emphasize that the ObjectLevelWorkloadIdentity feature gate " +
	"was retired in Flux, and the new field better expresses the effect of " +
	"both fields for Flux >=2.10: the field no longer sets the feature gate " +
	"and instead just adds the necessary RBAC for object identity to work. " +
	"object identity is the new name of the upstream Flux feature."

// multitenantWorkloadIdentityLegacyWarning is logged when the legacy
// MultitenantWorkloadIdentity field is used with Flux v2.10.0 or later.
const multitenantWorkloadIdentityLegacyWarning = "warning: in Flux >=2.10, " +
	"FluxInstance.spec.cluster.multitenantWorkloadIdentity is now considered " +
	"legacy and is redundant with the new FluxInstance.spec.cluster.multitenantObjectIdentity. " +
	"their behavior is identical, so you can just change to the new field. " +
	"this is to emphasize that object identity is the new name of the upstream " +
	"Flux feature, and workload identity refers only to identities assigned to " +
	"the Flux controllers."

// DefaultWorkloadIdentityServiceAccountLegacyWarning is logged when the legacy
// default workload identity service account flag or environment variable is used.
const DefaultWorkloadIdentityServiceAccountLegacyWarning = "warning: " +
	"the --default-workload-identity-service-account flag and the " +
	"DEFAULT_WORKLOAD_IDENTITY_SERVICE_ACCOUNT environment variable are now " +
	"considered legacy and are redundant with the " +
	"--default-object-identity-service-account flag and the " +
	"DEFAULT_OBJECT_IDENTITY_SERVICE_ACCOUNT environment variable. " +
	"their behavior is identical, so you can just change to the new flag or " +
	"environment variable. this is to emphasize that object identity is the " +
	"new name of the upstream Flux feature, and workload identity refers only " +
	"to identities assigned to the Flux controllers and Operator."

// objectLevelWorkloadIdentityOnce ensures the legacy warning for
// ObjectLevelWorkloadIdentity is logged only once per process.
var objectLevelWorkloadIdentityOnce sync.Once

// multitenantWorkloadIdentityOnce ensures the legacy warning for
// MultitenantWorkloadIdentity is logged only once per process.
var multitenantWorkloadIdentityOnce sync.Once

// ValidateAndApplyObjectIdentityConfig validates and applies the object identity options
// based on the provided cluster configuration and Flux version in the Options struct.
// objectIdentityViaSPIFFE indicates that object identity is provided by the SPIFFE
// Broker API instead of Kubernetes service account tokens.
func (o *Options) ValidateAndApplyObjectIdentityConfig(cluster fluxcdv1.Cluster, objectIdentityViaSPIFFE bool) error {
	logger := o.Logger
	if logger.GetSink() == nil {
		logger = logr.Discard()
	}

	firstControllersWithObjectIdentity := make([]string, 0, 6)
	firstControllersWithObjectIdentity = append(firstControllersWithObjectIdentity,
		"source-controller",
		"kustomize-controller",
		"notification-controller",
		"image-reflector-controller",
		"image-automation-controller",
	)
	allControllers := append(firstControllersWithObjectIdentity, "helm-controller")

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
	lt210, err := checkVersionAgainstConstraint(version, "< 2.10.0")
	if err != nil {
		return err
	}

	// ObjectIdentityRBAC behaves the same as ObjectLevelWorkloadIdentity.
	// ObjectLevelWorkloadIdentity is kept for backwards compatibility.
	objectIdentityEnabled := cluster.ObjectLevelWorkloadIdentity || cluster.ObjectIdentityRBAC

	// The SPIFFE Broker API is only supported in Flux >= 2.10.
	objectIdentityViaSPIFFE = objectIdentityViaSPIFFE && !lt210

	// MultitenantObjectIdentity behaves the same as MultitenantWorkloadIdentity.
	// MultitenantWorkloadIdentity is kept for backwards compatibility.
	multitenantWorkloadIdentityEnabled := cluster.MultitenantWorkloadIdentity
	multitenantObjectIdentityEnabled := cluster.MultitenantObjectIdentity
	multitenantObjectIdentityAny := multitenantWorkloadIdentityEnabled || multitenantObjectIdentityEnabled

	switch {

	// Handle less than 2.6.0 versions.
	case lt260:

		// Perform checks specific to this version range.
		if objectIdentityEnabled || multitenantObjectIdentityAny {
			return fmt.Errorf(".objectLevelWorkloadIdentity, .objectIdentityRBAC and .multitenantObjectIdentity are not supported in Flux versions < 2.6.0")
		}

		// Nothing to add, nothing to remove.
		return nil

	// Handle 2.6.x versions.
	case lt270:

		// Perform checks specific to this version range.
		if multitenantObjectIdentityAny {
			return fmt.Errorf(".multitenantObjectIdentity is not supported in Flux versions 2.6.x")
		}

		// In 2.6.x, if object identity is not enabled, we can
		// just remove the permission for creating service account tokens, as
		// the feature gate is opt-in in this version range and hence we don't
		// need to set it to false in this case.
		if !objectIdentityEnabled {
			o.RemovePermissionForCreatingServiceAccountTokens = true
			return nil
		}

		// If enabled, append the patch for enabling the
		// ObjectLevelWorkloadIdentity feature gate.
		const enabled = true
		o.Patches += GetProfileObjectLevelWorkloadIdentity(firstControllersWithObjectIdentity, enabled)
		return nil

	// Handle >= 2.7.0 and < 2.10.0 versions.
	case lt210:

		// Perform checks specific to this version range.
		if !objectIdentityEnabled && multitenantWorkloadIdentityEnabled {
			return fmt.Errorf(".objectIdentityRBAC or .objectLevelWorkloadIdentity must be set to true when .multitenantWorkloadIdentity is set to true")
		}
		if !objectIdentityEnabled && multitenantObjectIdentityEnabled {
			return fmt.Errorf(".objectIdentityRBAC must be set to true when .multitenantObjectIdentity is set to true")
		}

		// The feature gate may or may not be opt-in for a version in this range,
		// so we need to set it according to the cluster configuration in order
		// not to conflict with the default behavior.
		o.Patches += GetProfileObjectLevelWorkloadIdentity(allControllers, objectIdentityEnabled)

		// Handle object identity disabled.
		if !objectIdentityEnabled {
			o.RemovePermissionForCreatingServiceAccountTokens = true
			return nil
		}

		// Nothing else to do if multitenant object identity is disabled.
		if !multitenantObjectIdentityAny {
			return nil
		}

		// Multi-tenancy is enabled. Append default service account controller flags.
		o.Patches += GetProfileMultitenantWorkloadIdentity(cluster)
		return nil

	// Handle >= 2.10.0 versions.
	default:

		// Perform checks specific to this version range.
		if !objectIdentityEnabled && multitenantWorkloadIdentityEnabled {
			return fmt.Errorf(".objectIdentityRBAC or .objectLevelWorkloadIdentity must be set to true when .multitenantWorkloadIdentity is set to true")
		}
		if !objectIdentityEnabled && multitenantObjectIdentityEnabled && !objectIdentityViaSPIFFE {
			return fmt.Errorf(".objectIdentityRBAC must be set to true when .multitenantObjectIdentity is set to true and object identity is not provided by the SPIFFE Broker API")
		}

		// Warn that the legacy fields are redundant with the new fields.
		if cluster.ObjectLevelWorkloadIdentity {
			objectLevelWorkloadIdentityOnce.Do(func() {
				logger.Info(objectLevelWorkloadIdentityLegacyWarning)
			})
		}
		if cluster.MultitenantWorkloadIdentity {
			multitenantWorkloadIdentityOnce.Do(func() {
				logger.Info(multitenantWorkloadIdentityLegacyWarning)
			})
		}

		// The ObjectLevelWorkloadIdentity feature gate is retired, so it must
		// not be passed to the controllers anymore.
		if objectIdentityEnabled {
			// The RBAC for creating service account tokens is no longer part of
			// the Flux distribution starting with 2.10, so we need to add it back.
			o.AddPermissionForCreatingServiceAccountTokens = true
		}

		// Nothing else to do if multitenant object identity is disabled.
		if !multitenantObjectIdentityAny {
			return nil
		}

		// Multi-tenancy is enabled. Append default service account controller flags.
		o.Patches += GetProfileMultitenantWorkloadIdentity(cluster)
		return nil
	}
}
