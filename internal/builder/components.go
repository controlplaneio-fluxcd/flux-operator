// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"text/template"
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
	err := o.buildVersionInfo()
	if err != nil {
		return err
	}

	// Ensure that source-watcher is not enabled for versions < 2.7.0.
	if o.VersionInfo.Minor < 7 && o.HasSourceWatcher() {
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

		var data bytes.Buffer
		if err := t.Execute(&data, o); err != nil {
			return err
		}
		o.Patches += data.String()
	}

	return nil
}
