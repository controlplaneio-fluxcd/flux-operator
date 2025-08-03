// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// objectLevelWorkloadIdentity holds the configuration for the object level workload identity feature.
type objectLevelWorkloadIdentity struct {
	enabled     bool
	supported   bool
	controllers []string
}

// controllersForTemplate returns the regular expression for the kustomize patch in the template.
func (o *objectLevelWorkloadIdentity) controllersForTemplate() string {
	return fmt.Sprintf("(%s)", strings.Join(o.controllers, "|"))
}

// newObjectLevelWorkloadIdentity creates a new object level workload identity configuration
// depending on the Flux version and whether the feature is enabled.
func newObjectLevelWorkloadIdentity(fluxVersion string, objectLevelWorkloadID bool) (*objectLevelWorkloadIdentity, error) {
	// First, check if the Flux version supports the object level workload identity feature,
	// which means checking if the version is >= 2.6.0 (we check the opposite for the early return).
	lt260, err := semver.NewConstraint("< 2.6.0")
	if err != nil {
		return nil, fmt.Errorf("semver constraint parse error: %w", err)
	}
	version, err := semver.NewVersion(fluxVersion)
	if err != nil {
		return nil, fmt.Errorf("version '%s' parse error: %w", fluxVersion, err)
	}
	if lt260.Check(version) {
		return &objectLevelWorkloadIdentity{}, nil
	}

	o := &objectLevelWorkloadIdentity{
		supported: true,
		enabled:   objectLevelWorkloadID,
		controllers: []string{
			// Controllers that support object level workload identity since Flux >= 2.6.0.
			"source-controller",
			"kustomize-controller",
			"notification-controller",
			"image-reflector-controller",
			"image-automation-controller",
		},
	}

	// From the Flux version >= 2.7.0, helm-controller also supports object level workload identity.
	gte270, err := semver.NewConstraint(">= 2.7.0")
	if err != nil {
		return nil, fmt.Errorf("semver constraint parse error: %w", err)
	}
	if gte270.Check(version) {
		o.controllers = append(o.controllers, "helm-controller")
	}

	return o, nil
}
