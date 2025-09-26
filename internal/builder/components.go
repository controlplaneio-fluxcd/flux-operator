// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"text/template"

	"github.com/Masterminds/semver/v3"
)

// DefaultComponents defines the default set of Flux controllers.
var DefaultComponents = []string{
	"source-controller",
	"kustomize-controller",
	"helm-controller",
	"notification-controller",
}

// AllComponents defines all available Flux 2.7+ controllers.
var AllComponents = append(DefaultComponents,
	"image-reflector-controller",
	"image-automation-controller",
	"source-watcher",
)

// HasNotificationController returns true if the notification-controller
// is included in the selected components.
func (o *Options) HasNotificationController() bool {
	return slices.Contains(o.Components, "notification-controller")
}

// HasSourceWatcher returns true if the source-watcher
// is included in the selected components.
func (o *Options) HasSourceWatcher() bool {
	return slices.Contains(o.Components, "source-watcher")
}

// ValidateAndPatchComponents verifies that the selected components
// are valid and compatible with the specified Flux version.
// It also appends a patch to include the Flux Operator CRDs in the
// notification-controller configuration.
func (o *Options) ValidateAndPatchComponents() error {
	// Ensure at least one component is specified.
	if len(o.Components) == 0 {
		return errors.New("no components defined")
	}

	// Ensure all specified components are valid.
	for _, c := range o.Components {
		if !slices.Contains(AllComponents, c) {
			return fmt.Errorf("invalid component: %s", c)
		}
	}

	// Parse Flux version.
	ver, err := semver.NewVersion(o.Version)
	if err != nil {
		return fmt.Errorf("failed to parse Flux version '%s': %w", o.Version, err)
	}

	// Check if the version is less than 2.7.0 (source-watcher was introduced in 2.7.0).
	lt270, err := checkVersionAgainstConstraint(ver, "< 2.7.0")
	if err != nil {
		return err
	}

	// Ensure that source-watcher is not enabled for versions < 2.7.0.
	if lt270 && o.HasSourceWatcher() {
		return errors.New("source-watcher is only supported in Flux versions >= 2.7.0")
	}

	// Enable the ExternalArtifact feature gate if source-watcher is included.
	if o.HasSourceWatcher() {
		o.Patches += profileExternalArtifactFeatureGate
	}

	// Add the notification-controller patch if the component is included.
	// The patch adds the Flux Operator CRDs to the list of resources
	// that the notification-controller accepts for sending and receiving events.
	if o.HasNotificationController() {
		t, err := template.New("tmpl").Parse(notificationPatchTmpl)
		if err != nil {
			return err
		}

		obj := struct {
			Namespace string
			Lt270     bool
		}{
			Namespace: o.Namespace,
			Lt270:     lt270,
		}

		var data bytes.Buffer
		if err := t.Execute(&data, obj); err != nil {
			return err
		}
		o.Patches += data.String()
	}

	return nil
}
